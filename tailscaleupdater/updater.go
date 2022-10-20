package tailscaleupdater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type TailscaleAdvertisementUpdater struct {
	URL    string
	routes map[string]struct{}
}

// NewTailscaleAdvertisementUpdater creates an updater with the given URL if
// a GET request to the root path returns StatusOK
func NewTailscaleAdvertisementUpdater(url string) (*TailscaleAdvertisementUpdater, error) {
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
	}, nil
}

// SetupInformer creates a services informer on the given informer factory,
// and sets up a handler which updates the Tailscale route advertisements
func (t *TailscaleAdvertisementUpdater) SetupInformer(informerFactory informers.SharedInformerFactory) cache.SharedIndexInformer {
	servicesInformer := informerFactory.Core().V1().Services().Informer()

	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			fmt.Println("Not a service?")
			return
		}
		fmt.Println("discovered service", svc.Name, svc.Spec.ClusterIP)
		err := t.ensureRouteForIP(svc.Spec.ClusterIP)
		if err != nil {
			fmt.Println("error adding route for svc:", err)
		}
	}

	handler.UpdateFunc = func(old, new interface{}) {
		oldsvc, ok := old.(*corev1.Service)
		if !ok {
			fmt.Println("Not a service?")
			return
		}
		newsvc, ok := new.(*corev1.Service)
		if !ok {
			fmt.Println("Not a service?")
			return
		}
		if oldsvc.Spec.ClusterIP != newsvc.Spec.ClusterIP {
			fmt.Println("ip updated for service", oldsvc.Name)
			err := t.updateRoute(oldsvc.Spec.ClusterIP, newsvc.Spec.ClusterIP)
			if err != nil {
				fmt.Println("error updating route for svc:", err)
			}
		}
	}

	handler.DeleteFunc = func(obj interface{}) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			fmt.Println("Not a service?")
			return
		}
		fmt.Println("service", svc.Name, "removed")
		err := t.removeRouteForIP(svc.Spec.ClusterIP)
		if err != nil {
			fmt.Println("error removing route for svc:", err)
		}
	}
	servicesInformer.AddEventHandler(handler)

	return servicesInformer
}

// ensureRouteForIP registers a /32 route for the provided ip if it doesn't
// exist in the updater, and posts the new advertisments to the tailscale API.
func (t *TailscaleAdvertisementUpdater) ensureRouteForIP(ip string) error {
	if t.addRoute(ip) {
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
	added := t.addRoute(newip)
	if removed || added {
		return t.post()
	}
	return nil
}

// addRoute adds route for ip to internal state, returns true if
// advertisements need to be updated
func (t *TailscaleAdvertisementUpdater) addRoute(ip string) bool {
	new_route := ip + "/32"
	if _, ok := t.routes[new_route]; ok {
		return false
	}
	fmt.Println("Adding route", new_route)
	t.routes[new_route] = struct{}{}
	return true
}

// removeRoute removes route for ip from internal state, returns true if
// advertisements need to be updated
func (t *TailscaleAdvertisementUpdater) removeRoute(ip string) bool {
	route := ip + "/32"
	if _, ok := t.routes[route]; ok {
		fmt.Println("Removing route", route)
		delete(t.routes, route)
		return true
	}
	return false
}

// post generates the API request payload and posts the current
// routeAdvertisements to the Tailscale API
func (t *TailscaleAdvertisementUpdater) post() error {
	routes := []string{}
	for r, _ := range t.routes {
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
