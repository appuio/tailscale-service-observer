package tailscaleupdater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TailscaleAdvertisementUpdater keeps track of the routes which are
// advertised through the Tailscale client.
type TailscaleAdvertisementUpdater struct {
	URL    string
	routes map[string]struct{}
	logger logr.Logger
}

// New creates a new updater with the given URL if a GET request to the root
// path returns StatusOK
func New(namespaces []string, url string) (*TailscaleAdvertisementUpdater, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Error querying Tailscale API at %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tailscale API at %s responded with status '%s'", url, resp.Status)
	}
	return &TailscaleAdvertisementUpdater{
		URL:    url,
		routes: map[string]struct{}{},
		logger: ctrl.Log.WithName("tailscaleUpdater").WithValues("API URL", url, "namespaces", strings.Join(namespaces, ",")),
	}, nil
}

// SetupInformer creates a services informer on the given informer factory,
// and sets up a handler which updates the Tailscale route advertisements
func (t *TailscaleAdvertisementUpdater) SetupInformer(informerFactory informers.SharedInformerFactory) cache.SharedIndexInformer {
	servicesInformer := informerFactory.Core().V1().Services().Informer()

	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    t.informerAddHandler,
		UpdateFunc: t.informerUpdateHandler,
		DeleteFunc: t.informerDeleteHandler,
	}

	servicesInformer.AddEventHandler(handler)

	return servicesInformer
}

func (t *TailscaleAdvertisementUpdater) AddRoute(route string) error {
	if t.addRoute(route) {
		return t.post()
	}
	return nil
}

func (t *TailscaleAdvertisementUpdater) informerAddHandler(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		t.logger.V(1).Info("add: got non-service object")
		return
	}
	svcIP := svc.Spec.ClusterIP
	t.logger.Info("discovered service", "name", svc.Name, "ip", svcIP)
	err := t.ensureRouteForIP(svcIP)
	if err != nil {
		t.logger.Error(err, "adding route for service")
	}
}

func (t *TailscaleAdvertisementUpdater) informerUpdateHandler(old, new interface{}) {
	oldsvc, ok := old.(*corev1.Service)
	if !ok {
		t.logger.V(1).Info("update: got old non-service")
		return
	}
	newsvc, ok := new.(*corev1.Service)
	if !ok {
		t.logger.V(1).Info("update: got new non-service")
		return
	}
	if oldsvc.Spec.ClusterIP != newsvc.Spec.ClusterIP {
		oldIP := oldsvc.Spec.ClusterIP
		newIP := newsvc.Spec.ClusterIP
		t.logger.Info("ip updated for service", "name", oldsvc.Name, "old ip", oldIP, "new ip", newIP)
		err := t.updateRoute(oldIP, newIP)
		if err != nil {
			t.logger.Error(err, "updating route for service")
		}
	}
}

func (t *TailscaleAdvertisementUpdater) informerDeleteHandler(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		t.logger.V(1).Info("delete: non-service object")
		return
	}
	svcIP := svc.Spec.ClusterIP
	t.logger.Info("service removed", "name", svc.Name, "ip", svcIP)
	err := t.removeRouteForIP(svcIP)
	if err != nil {
		t.logger.Error(err, "removing route for service")
	}
}

// ensureRouteForIP registers a /32 route for the provided ip if it doesn't
// exist in the updater, and posts the new advertisments to the tailscale API.
func (t *TailscaleAdvertisementUpdater) ensureRouteForIP(ip string) error {
	if t.addRouteForIP(ip) {
		return t.post()
	}
	return nil
}

// removeRouteForIP removes the /32 route for the provided ip if it's
// registered in the updater, and posts the new advertisements to the
// tailscale API.
func (t *TailscaleAdvertisementUpdater) removeRouteForIP(ip string) error {
	if t.removeRoute(ip) {
		return t.post()
	}
	return nil
}

// updateRoute removes the route for oldip if it exists in the state, adds the
// route for newip if it's missing in the state, and posts the new
// advertisements if the internal state changed
func (t *TailscaleAdvertisementUpdater) updateRoute(oldip string, newip string) error {
	removed := t.removeRoute(oldip)
	added := t.addRouteForIP(newip)
	if removed || added {
		return t.post()
	}
	return nil
}

// addRouteForIP adds route for ip to internal state, returns true if
// advertisements need to be updated
func (t *TailscaleAdvertisementUpdater) addRouteForIP(ip string) bool {
	newRoute := ip + "/32"
	return t.addRoute(newRoute)
}

// addRoute adds a route to the internal state, returns true if
// advertisements need to be updated
func (t *TailscaleAdvertisementUpdater) addRoute(route string) bool {
	if _, ok := t.routes[route]; ok {
		return false
	}
	t.logger.Info("adding", "route", route)
	t.routes[route] = struct{}{}
	return true
}

// removeRoute removes route for ip from internal state, returns true if
// advertisements need to be updated
func (t *TailscaleAdvertisementUpdater) removeRoute(ip string) bool {
	route := ip + "/32"
	if _, ok := t.routes[route]; ok {
		t.logger.Info("removing", "route", route)
		delete(t.routes, route)
		return true
	}
	return false
}

// post generates the API request payload and posts the current
// routeAdvertisements to the Tailscale API
func (t *TailscaleAdvertisementUpdater) post() error {
	routes := []string{}
	for r := range t.routes {
		routes = append(routes, r)
	}
	payload := struct {
		AdvertiseRoutes string `json:"advertiseRoutes"`
	}{
		AdvertiseRoutes: strings.Join(routes, ","),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(t.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
