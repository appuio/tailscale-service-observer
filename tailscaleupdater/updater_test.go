package tailscaleupdater

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_NewTailscaleAdvertisementUpdater(t *testing.T) {
	url := tsAPIServer()

	tsUpdater, err := NewTailscaleAdvertisementUpdater([]string{"foo"}, url)
	assert.NoError(t, err)
	assert.Equal(t, url, tsUpdater.URL)
}

func Test_NewTailscaleAdvertisementUpdater_NoService(t *testing.T) {
	tsUpdater, err := NewTailscaleAdvertisementUpdater([]string{"foo"}, "")
	assert.Error(t, err)
	assert.Nil(t, tsUpdater)
}

func Test_SetupInformer(t *testing.T) {
	tsUpdater := mockUpdater(t)

	client := mockClient([]runtime.Object{})
	factory := informers.NewSharedInformerFactoryWithOptions(client, 10*time.Minute, informers.WithNamespace("foo"))

	informer := tsUpdater.SetupInformer(factory)

	assert.NotNil(t, informer)
}

func Test_informerAddHandler(t *testing.T) {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "198.51.100.1",
		},
	}
	tsUpdater := mockUpdater(t)

	tsUpdater.informerAddHandler(&svc)

	assert.Equal(t, map[string]struct{}{"198.51.100.1/32": {}}, tsUpdater.routes)

}

func Test_informerAddHandler_WrongKind(t *testing.T) {
	obj := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	tsUpdater := mockUpdater(t)

	tsUpdater.informerAddHandler(&obj)

	assert.Equal(t, map[string]struct{}{}, tsUpdater.routes)
}

func Test_informerUpdateHandler(t *testing.T) {
	tcases := map[string]struct {
		initialRoutes  map[string]struct{}
		expectedRoutes map[string]struct{}
		oldServiceIP   string
		newServiceIP   string
	}{
		"NoIPChange": {
			initialRoutes: map[string]struct{}{
				"198.51.100.1/32": {},
			},
			expectedRoutes: map[string]struct{}{
				"198.51.100.1/32": {},
			},
			oldServiceIP: "198.51.100.1",
			newServiceIP: "198.51.100.1",
		},
		"IPChange": {
			initialRoutes: map[string]struct{}{
				"198.51.100.1/32": {},
			},
			expectedRoutes: map[string]struct{}{
				"198.51.100.2/32": {},
			},
			oldServiceIP: "198.51.100.1",
			newServiceIP: "198.51.100.2",
		},
		"IPAdd": {
			initialRoutes: map[string]struct{}{},
			expectedRoutes: map[string]struct{}{
				"198.51.100.2/32": {},
			},
			oldServiceIP: "198.51.100.1",
			newServiceIP: "198.51.100.2",
		},
		"IPRemove": {
			initialRoutes: map[string]struct{}{
				"198.51.100.1/32": {},
				"198.51.100.2/32": {},
			},
			expectedRoutes: map[string]struct{}{
				"198.51.100.2/32": {},
			},
			oldServiceIP: "198.51.100.1",
			newServiceIP: "198.51.100.2",
		},
	}
	for _, tc := range tcases {
		oldSvc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: tc.oldServiceIP,
			},
		}
		newSvc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: tc.newServiceIP,
			},
		}
		tsUpdater := mockUpdater(t)
		tsUpdater.routes = tc.initialRoutes

		tsUpdater.informerUpdateHandler(&oldSvc, &newSvc)

		assert.Equal(t, tc.expectedRoutes, tsUpdater.routes)
	}

}

func Test_informerUpdateHandler_WrongKind(t *testing.T) {
	tcases := map[string]struct {
		old runtime.Object
		new runtime.Object
	}{
		"OldWrongKind": {
			old: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			new: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
		},
		"NewWrongKind": {
			old: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			new: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
		},
	}
	for _, tc := range tcases {
		tsUpdater := mockUpdater(t)

		tsUpdater.informerUpdateHandler(tc.old, tc.new)

		assert.Equal(t, map[string]struct{}{}, tsUpdater.routes)
	}
}

func Test_informerDeleteHandler(t *testing.T) {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "198.51.100.1",
		},
	}
	tsUpdater := mockUpdater(t)
	tsUpdater.routes["198.51.100.1/32"] = struct{}{}

	tsUpdater.informerDeleteHandler(&svc)

	assert.Equal(t, map[string]struct{}{}, tsUpdater.routes)

}

func Test_informerDeleteHandler_WrongKind(t *testing.T) {
	obj := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	tsUpdater := mockUpdater(t)
	tsUpdater.routes["198.51.100.1/32"] = struct{}{}

	tsUpdater.informerDeleteHandler(&obj)

	assert.Equal(t, map[string]struct{}{"198.51.100.1/32": {}}, tsUpdater.routes)
}

func mockUpdater(t *testing.T) *TailscaleAdvertisementUpdater {
	return &TailscaleAdvertisementUpdater{
		URL:    "foobar",
		routes: map[string]struct{}{},
		logger: testr.New(t),
	}
}

func mockClient(objects []runtime.Object) *fake.Clientset {
	return fake.NewSimpleClientset(objects...)
}

func tsAPIServer() string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	return server.URL
}
