package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/appuio/tailscale-service-observer/tailscaleupdater"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// these variables are populated by Goreleaser when releasing
	version = "unknown"
	commit  = "-dirty-"
	date    = time.Now().Format("2006-01-02")

	appName     = "tailscale-service-observer"
	appLongName = "A tool which watches Kubernetes services and updates Tailscale route advertisements through the Tailscale client's HTTP API"
)

// createClient creates a new Kubernetes client either from the current
// kubeconfig context, or from the in-cluster config if the program is running
// in a pod.
func createClient() (*kubernetes.Clientset, error) {
	// if you want to change the loading rules (which files in which order), you can do so here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// if you want to change override values or bind them to flags, there are methods to help you
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func main() {
	// TODO(sg): setup logging
	ctx, cancelFunc := context.WithCancel(context.Background())

	// setup exit on SIGTERM, SIGINT
	exitSignal := make(chan os.Signal)
	// setup stop channel for K8s informers
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		var signum = -1

		receivedSignal := <-exitSignal

		if sysSignal, ok := receivedSignal.(syscall.Signal); ok {
			signum = int(sysSignal)
		}

		fmt.Printf("Received signal %d (%s)\n", signum, receivedSignal.String())

		cancelFunc()
	}()

	client, err := createClient()
	if err != nil {
		cancelFunc()
		fmt.Println("Error setting up client:", err)
		os.Exit(1)
	}

	targetNamespace, ok := os.LookupEnv("TARGET_NAMESPACE")
	if !ok {
		fmt.Println("Unable to read target namespace from environment ($TARGET_NAMESPACE)")
		os.Exit(1)
	}

	tsApiURL, ok := os.LookupEnv("TAILSCALE_API_URL")
	if !ok {
		fmt.Println("Unable to read Tailscale client API URL from environment ($TAILSCALE_API_URL)")
		os.Exit(1)
	}
	tsUpdater, err := tailscaleupdater.NewTailscaleAdvertisementUpdater(tsApiURL)
	if err != nil {
		fmt.Println("Error while creating Tailscale updater:", err)
		os.Exit(1)
	}

	factory := informers.NewSharedInformerFactoryWithOptions(client, 10*time.Minute, informers.WithNamespace(targetNamespace))
	_ = tsUpdater.SetupInformer(factory)

	// start informers
	factory.Start(ctx.Done()) // runs in background
	factory.WaitForCacheSync(ctx.Done())

	<-ctx.Done()
}
