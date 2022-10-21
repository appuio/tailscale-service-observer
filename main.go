package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/appuio/tailscale-service-observer/tailscaleupdater"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	// these variables are populated by Goreleaser when releasing
	version = "unknown"
	commit  = "-dirty-"
	date    = time.Now().Format("2006-01-02")

	appName = "tailscale-service-observer"

	DefaultTailscaleApiURL = "http://localhost:8088"
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
	// use controller-runtime to setup logging and context
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	setupLog := ctrl.Log.WithName("setup")

	rawTargetNamespace, ok := os.LookupEnv("TARGET_NAMESPACE")
	if !ok {
		setupLog.Error(fmt.Errorf("TARGET_NAMESPACE not set"), "Unable to read target namespace from environment ($TARGET_NAMESPACE)")
		os.Exit(1)
	}
	targetNamespaces := strings.Split(rawTargetNamespace, ",")

	var tsApiURL string
	tsApiURL, ok = os.LookupEnv("TAILSCALE_API_URL")
	if !ok {
		tsApiURL = DefaultTailscaleApiURL
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctx := ctrl.SetupSignalHandler()

	client, err := createClient()
	if err != nil {
		setupLog.Error(err, "setting up Kubernetes client")
		os.Exit(1)
	}

	tsUpdater, err := tailscaleupdater.NewTailscaleAdvertisementUpdater(targetNamespaces, tsApiURL)
	if err != nil {
		setupLog.Error(err, "while creating Tailscale updater")
		os.Exit(1)
	}

	for _, ns := range targetNamespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(client, 10*time.Minute, informers.WithNamespace(ns))
		_ = tsUpdater.SetupInformer(factory)

		// start informers
		factory.Start(ctx.Done()) // runs in background
		factory.WaitForCacheSync(ctx.Done())
	}

	<-ctx.Done()
}
