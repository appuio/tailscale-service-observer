package main

import (
	"flag"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/appuio/tailscale-service-observer/tailscaleupdater"
	"github.com/go-logr/logr"
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

	defaultTailscaleAPIURL = "http://localhost:8088"
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

// parseEnv parses comma-separated values from an env variable
func parseEnv(raw string) []string {
	parts := strings.Split(raw, ",")
	parsed := []string{}
	for _, ns := range parts {
		trimmed := strings.Trim(ns, " ")
		if trimmed != "" {
			parsed = append(parsed, trimmed)
		}
	}
	return parsed
}

func advertiseAdditionalRoutes(l logr.Logger, t *tailscaleupdater.TailscaleAdvertisementUpdater, rawRoutes string) {
	for _, rspec := range parseEnv(rawRoutes) {
		r, err := netip.ParsePrefix(rspec)
		if err != nil {
			a, err2 := netip.ParseAddr(rspec)
			if err2 == nil {
				r2, err2 := a.Prefix(a.BitLen())
				if err2 != nil {
					l.Error(err, "converting bare IP to prefix")
				}
				r = r2
				err = err2
			}
		}
		if err != nil {
			l.Info("Failed to parse additional route, ignoring", "value", rspec)
			continue
		}
		if err := t.AddRoute(r.String()); err != nil {
			l.Error(err, "adding additional route", "route", r.String())
		}
	}
}

func main() {
	// use controller-runtime to setup logging and context
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctx := ctrl.SetupSignalHandler()
	setupLog := ctrl.Log.WithName("setup")

	rawTargetNamespace, ok := os.LookupEnv("TARGET_NAMESPACE")
	if !ok {
		setupLog.Info("Unable to read target namespace from environment ($TARGET_NAMESPACE)")
		os.Exit(1)
	}
	targetNamespaces := parseEnv(rawTargetNamespace)

	var apiURL string
	apiURL, ok = os.LookupEnv("TAILSCALE_API_URL")
	if !ok {
		apiURL = defaultTailscaleAPIURL
	}
	tsUpdater, err := tailscaleupdater.New(targetNamespaces, apiURL)

	additionalRoutes, ok := os.LookupEnv("OBSERVER_ADDITIONAL_ROUTES")
	if ok {
		advertiseAdditionalRoutes(setupLog, tsUpdater, additionalRoutes)
	}

	client, err := createClient()
	if err != nil {
		setupLog.Error(err, "setting up Kubernetes client")
		os.Exit(1)
	}

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
