/*
Copyright 2026 Metacontroller authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package informer

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	"metacontroller/pkg/internal/testutils/dynamic/discovery"
)

const (
	testGroup      = "example.com"
	testVersion    = "v1"
	testResource   = "widgets"
	testKind       = "Widget"
	testNamespace  = "default"
	testAPIVersion = testGroup + "/" + testVersion

	// waitLimit is the longest time that a test waits for an expected event.
	waitLimit = 5 * time.Second
	// quietWindow is the time in which a test looks for an event that must
	// not come. A longer window does not make such a test more correct.
	quietWindow = 100 * time.Millisecond
	// shortResync is a handler resync period below the informer period.
	shortResync = 20 * time.Millisecond
)

var testGVR = schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}

func TestListByOwnerUID_whenObjectHasControllerOwner_returnsOwnedObject(t *testing.T) {
	const ownerUID = types.UID("parent-uid")
	ti := newTestInformer(t, newWidget(testNamespace, "owned", ownerUID))
	ri := newResourceInformer(ti.sri)

	owned, err := ri.ListByOwnerUID(ownerUID, testNamespace)

	require.NoError(t, err)
	require.Len(t, owned, 1)
	assert.Equal(t, "owned", owned[0].GetName())
}

func TestListByOwnerUID_whenObjectIsInAnotherNamespace_doesNotReturnIt(t *testing.T) {
	const ownerUID = types.UID("parent-uid")
	ti := newTestInformer(t, newWidget("other", "owned", ownerUID))
	ri := newResourceInformer(ti.sri)

	owned, err := ri.ListByOwnerUID(ownerUID, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, owned)
}

func TestListOrphans_whenObjectHasNoControllerOwner_returnsOrphan(t *testing.T) {
	ti := newTestInformer(t,
		newWidget(testNamespace, "orphan", ""),
		newWidget(testNamespace, "owned", types.UID("parent-uid")),
	)
	ri := newResourceInformer(ti.sri)

	orphans, err := ri.ListOrphans(testNamespace)

	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, "orphan", orphans[0].GetName())
}

// This test guards a client-go fact. See CHECK AFTER A client-go UPDATE in
// .agents/dynamic-client.md.
func TestAddEventHandler_whenObjectExists_replaysItAsInitialAdd(t *testing.T) {
	ti := newTestInformer(t, newWidget(testNamespace, "existing", ""))
	ri := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}

	_, err := ri.Informer().AddEventHandler(recorder.handler())
	t.Cleanup(ri.Informer().RemoveEventHandlers)

	require.NoError(t, err)
	// client-go delivers the events of one handler in order. So when the
	// handler has the new object, it has the full replay.
	ti.create(t, "created")
	recorder.waitForAdd(t, "created")
	assert.Equal(t, []string{"existing", "created"}, recorder.addNames())
	assert.Equal(t, []bool{true, false}, recorder.initialFlags())
	assert.Zero(t, recorder.updates(), "the replay must not come as an update")
}

func TestSkipInitialList_whenObjectIsInInitialList_doesNotCallAdd(t *testing.T) {
	called := false
	add := SkipInitialList(func(interface{}) { called = true })

	add(newWidget(testNamespace, "widget", ""), true)

	assert.False(t, called)
}

func TestSkipInitialList_whenObjectIsNotInInitialList_callsAdd(t *testing.T) {
	called := false
	add := SkipInitialList(func(interface{}) { called = true })

	add(newWidget(testNamespace, "widget", ""), false)

	assert.True(t, called)
}

// This test guards a client-go fact. See CHECK AFTER A client-go UPDATE in
// .agents/dynamic-client.md.
func TestSkipInitialList_whenObjectArrivesInALaterRelist_deliversOnlyThatObject(t *testing.T) {
	ti := newUnstartedTestInformer(t, time.Hour, newWidget(testNamespace, "existing", ""))
	// The first watch is not connected to the object tracker. So the informer
	// can see a new object only when it lists again.
	firstWatch := watch.NewRaceFreeFake()
	var watchCalls atomic.Int32
	ti.dc.PrependWatchReactor("*", func(clienttesting.Action) (bool, watch.Interface, error) {
		if watchCalls.Add(1) == 1 {
			return true, firstWatch, nil
		}
		return false, nil, nil
	})
	ti.start(t)
	ri := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}
	_, err := ri.Informer().AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: SkipInitialList(recorder.add),
	})
	require.NoError(t, err)
	t.Cleanup(ri.Informer().RemoveEventHandlers)
	ti.create(t, "created")

	// An expired watch makes the reflector list again.
	firstWatch.Error(&metav1.Status{
		Status: metav1.StatusFailure,
		Reason: metav1.StatusReasonExpired,
		Code:   http.StatusGone,
	})

	// Only the first list is the initial list. An object from a later list
	// must reach the handler, or a watch gap would lose it.
	recorder.waitForAdd(t, "created")
	assert.Equal(t, []string{"created"}, recorder.addNames(), "only the object of the later list must reach the handler")
}

func TestAddEventHandlerWithResyncPeriod_whenPeriodIsBelowDefault_repeatsResync(t *testing.T) {
	period := shortResync
	adders := map[string]func(SharedIndexInformer, cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error){
		"AddEventHandlerWithResyncPeriod": func(informer SharedIndexInformer, handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
			return informer.AddEventHandlerWithResyncPeriod(handler, period)
		},
		"AddEventHandlerWithOptions": func(informer SharedIndexInformer, handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
			return informer.AddEventHandlerWithOptions(handler, cache.HandlerOptions{ResyncPeriod: &period})
		},
	}
	for name, add := range adders {
		t.Run(name, func(t *testing.T) {
			ti := newTestInformer(t, newWidget(testNamespace, "existing", ""))
			ri := newResourceInformer(ti.sri)
			recorder := &eventRecorder{}

			_, err := add(ri.Informer(), recorder.handler())
			t.Cleanup(ri.Informer().RemoveEventHandlers)

			require.NoError(t, err)
			// The informer period is one hour, so each update comes from the
			// local timer.
			recorder.waitForUpdates(t, 2)
		})
	}
}

func TestAddEventHandlerWithResyncPeriod_whenPeriodIsZero_startsNoLocalTimer(t *testing.T) {
	ti := newTestInformer(t, newWidget(testNamespace, "existing", ""))
	ri := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}

	_, err := ri.Informer().AddEventHandlerWithResyncPeriod(recorder.handler(), 0)
	t.Cleanup(ri.Informer().RemoveEventHandlers)

	require.NoError(t, err)
	assert.Never(t, func() bool { return recorder.updates() != 0 }, quietWindow, 10*time.Millisecond)
}

func TestAddEventHandlerWithResyncPeriod_whenDefaultPeriodIsZero_startsNoTimer(t *testing.T) {
	// A zero default period disables resync. This is the same as before the
	// change to the client-go informer.
	ti := newUnstartedTestInformer(t, 0, newWidget(testNamespace, "existing", ""))
	ti.start(t)
	ri := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}

	_, err := ri.Informer().AddEventHandlerWithResyncPeriod(recorder.handler(), shortResync)
	t.Cleanup(ri.Informer().RemoveEventHandlers)

	require.NoError(t, err)
	assert.Never(t, func() bool { return recorder.updates() != 0 }, quietWindow, 10*time.Millisecond)
}

func TestAddEventHandler_whenAnotherHandlerIsBlocked_stillDeliversEvents(t *testing.T) {
	ti := newTestInformer(t)
	slow := newResourceInformer(ti.sri)
	fast := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}
	_, err := fast.Informer().AddEventHandler(recorder.handler())
	require.NoError(t, err)
	// Cleanup functions run in reverse order. The release must run first, or
	// a removal waits for the blocked handler.
	t.Cleanup(slow.Informer().RemoveEventHandlers)
	t.Cleanup(fast.Informer().RemoveEventHandlers)
	release := addBlockedHandler(t, ti, slow)
	t.Cleanup(release)

	ti.create(t, "created")

	recorder.waitForAdd(t, "created")
}

func TestAddEventHandler_whenRemovalWaitsForRunningHandler_doesNotBlock(t *testing.T) {
	ti := newTestInformer(t)
	ri := newResourceInformer(ti.sri)
	release := addBlockedHandler(t, ti, ri)
	removed := make(chan struct{})
	go func() {
		ri.Informer().RemoveEventHandlers()
		close(removed)
	}()
	waitUntilRemovalTookHandlers(t, ri)

	// RemoveEventHandlers does not hold the wrapper lock while it waits.
	added := make(chan error, 1)
	go func() {
		_, err := ri.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{})
		added <- err
	}()

	select {
	case err := <-added:
		require.NoError(t, err)
	case <-time.After(waitLimit):
		t.Fatal("AddEventHandler blocked while RemoveEventHandlers waited")
	}
	release()
	waitForClose(t, removed, "RemoveEventHandlers did not return after the handler finished")
}

func TestRemoveEventHandlers_whenSharedInformerHasOtherSubscribers_keepsTheirHandlers(t *testing.T) {
	ti := newTestInformer(t)
	first := newResourceInformer(ti.sri)
	second := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}
	_, err := first.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{})
	require.NoError(t, err)
	_, err = second.Informer().AddEventHandler(recorder.handler())
	require.NoError(t, err)
	t.Cleanup(second.Informer().RemoveEventHandlers)

	first.Informer().RemoveEventHandlers()

	ti.create(t, "created")
	recorder.waitForAdd(t, "created")
}

func TestRemoveEventHandlers_whenHandlerWasAdded_stopsDelivery(t *testing.T) {
	adders := map[string]func(SharedIndexInformer, cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error){
		"AddEventHandler": func(informer SharedIndexInformer, handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
			return informer.AddEventHandler(handler)
		},
		"AddEventHandlerWithResyncPeriod": func(informer SharedIndexInformer, handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
			return informer.AddEventHandlerWithResyncPeriod(handler, time.Minute)
		},
		"AddEventHandlerWithOptions": func(informer SharedIndexInformer, handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
			return informer.AddEventHandlerWithOptions(handler, cache.HandlerOptions{})
		},
	}
	for name, add := range adders {
		t.Run(name, func(t *testing.T) {
			ti := newTestInformer(t, newWidget(testNamespace, "existing", ""))
			ri := newResourceInformer(ti.sri)
			recorder := &eventRecorder{}
			_, err := add(ri.Informer(), recorder.handler())
			require.NoError(t, err)
			recorder.waitForAdd(t, "existing")

			ri.Informer().RemoveEventHandlers()

			// RemoveEventHandlers waits until the handler stops, so a late
			// event would show a handler that is still registered.
			ti.create(t, "created")
			assert.Never(t, func() bool {
				return slices.Contains(recorder.addNames(), "created")
			}, quietWindow, 10*time.Millisecond, "a removed handler must not get new events")
		})
	}
}

func TestRemoveEventHandlers_whenHandlerHasResyncTimer_stopsTheTimer(t *testing.T) {
	ti := newTestInformer(t, newWidget(testNamespace, "existing", ""))
	ri := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}
	_, err := ri.Informer().AddEventHandlerWithResyncPeriod(recorder.handler(), shortResync)
	require.NoError(t, err)
	recorder.waitForUpdates(t, 2)

	ri.Informer().RemoveEventHandlers()

	baseline := recorder.updates()
	assert.Never(t, func() bool { return recorder.updates() != baseline }, quietWindow, 10*time.Millisecond, "the resync timer must stop with the handler")
}

// This test guards a client-go fact. See CHECK AFTER A client-go UPDATE in
// .agents/dynamic-client.md.
func TestRemoveEventHandlers_whenHandlerIsRunning_waitsForHandler(t *testing.T) {
	ti := newTestInformer(t)
	ri := newResourceInformer(ti.sri)
	release := addBlockedHandler(t, ti, ri)

	removed := make(chan struct{})
	go func() {
		ri.Informer().RemoveEventHandlers()
		close(removed)
	}()

	select {
	case <-removed:
		t.Fatal("RemoveEventHandlers returned while a handler was still running")
	case <-time.After(quietWindow):
	}
	release()
	waitForClose(t, removed, "RemoveEventHandlers did not return after the handler finished")
}

func TestRemoveEventHandlers_whenAnotherRemovalWaits_waitsForTheSameHandlers(t *testing.T) {
	ti := newTestInformer(t)
	ri := newResourceInformer(ti.sri)
	release := addBlockedHandler(t, ti, ri)
	first := make(chan struct{})
	go func() {
		ri.Informer().RemoveEventHandlers()
		close(first)
	}()
	waitUntilRemovalTookHandlers(t, ri)

	second := make(chan struct{})
	go func() {
		ri.Informer().RemoveEventHandlers()
		close(second)
	}()

	select {
	case <-second:
		t.Fatal("the second removal returned while a handler was still running")
	case <-time.After(quietWindow):
	}
	release()
	waitForClose(t, first, "the first removal did not return after the handler finished")
	waitForClose(t, second, "the second removal did not return after the handler finished")
}

// This test guards a client-go fact. See CHECK AFTER A client-go UPDATE in
// .agents/dynamic-client.md.
func TestRemoveEventHandlers_whenInformerHasNotStarted_returnsWithoutWaiting(t *testing.T) {
	ti := newUnstartedTestInformer(t, time.Hour)
	ri := newResourceInformer(ti.sri)
	_, err := ri.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{})
	require.NoError(t, err)

	removed := make(chan struct{})
	go func() {
		ri.Informer().RemoveEventHandlers()
		close(removed)
	}()

	waitForClose(t, removed, "RemoveEventHandlers blocked for a handler that never started")
}

// This test guards a client-go fact. See CHECK AFTER A client-go UPDATE in
// .agents/dynamic-client.md.
func TestRemoveEventHandlers_whenInformerHasStopped_returns(t *testing.T) {
	ti := newUnstartedTestInformer(t, time.Hour, newWidget(testNamespace, "existing", ""))
	ri := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}
	_, err := ri.Informer().AddEventHandler(recorder.handler())
	require.NoError(t, err)
	stopCh := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		ti.sri.informer.Run(stopCh)
		close(stopped)
	}()
	recorder.waitForAdd(t, "existing")
	close(stopCh)
	waitForClose(t, stopped, "the informer did not stop")

	removed := make(chan struct{})
	go func() {
		ri.Informer().RemoveEventHandlers()
		close(removed)
	}()

	waitForClose(t, removed, "RemoveEventHandlers blocked after the informer stopped")
}

func TestRemoveEventHandler_whenHandlerHasResyncTimer_stopsOnlyThatTimer(t *testing.T) {
	ti := newTestInformer(t, newWidget(testNamespace, "existing", ""))
	ri := newResourceInformer(ti.sri)
	removed := &eventRecorder{}
	kept := &eventRecorder{}
	removedRegistration, err := ri.Informer().AddEventHandlerWithResyncPeriod(removed.handler(), shortResync)
	require.NoError(t, err)
	_, err = ri.Informer().AddEventHandlerWithResyncPeriod(kept.handler(), shortResync)
	require.NoError(t, err)
	t.Cleanup(ri.Informer().RemoveEventHandlers)
	removed.waitForUpdates(t, 2)

	err = ri.Informer().RemoveEventHandler(removedRegistration)

	require.NoError(t, err)
	baseline := removed.updates()
	assert.Never(t, func() bool { return removed.updates() != baseline }, quietWindow, 10*time.Millisecond, "a removed handler must not resync")
	kept.waitForUpdates(t, kept.updates()+1)
}

func TestRemoveEventHandler_whenRegistrationBelongsToAnotherSubscriber_keepsTheHandler(t *testing.T) {
	ti := newTestInformer(t)
	owner := newResourceInformer(ti.sri)
	other := newResourceInformer(ti.sri)
	recorder := &eventRecorder{}
	registration, err := owner.Informer().AddEventHandler(recorder.handler())
	require.NoError(t, err)
	t.Cleanup(owner.Informer().RemoveEventHandlers)

	err = other.Informer().RemoveEventHandler(registration)

	require.NoError(t, err)
	ti.create(t, "created")
	recorder.waitForAdd(t, "created")
}

func TestRemoveEventHandler_whenRegistrationIsNil_returnsError(t *testing.T) {
	ti := newUnstartedTestInformer(t, time.Hour)
	ri := newResourceInformer(ti.sri)

	err := ri.Informer().RemoveEventHandler(nil)

	assert.Error(t, err)
}

func TestRemoveEventHandler_whenCalledTwice_returnsNoError(t *testing.T) {
	ti := newUnstartedTestInformer(t, time.Hour)
	ri := newResourceInformer(ti.sri)
	registration, err := ri.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{})
	require.NoError(t, err)
	require.NoError(t, ri.Informer().RemoveEventHandler(registration))

	err = ri.Informer().RemoveEventHandler(registration)

	assert.NoError(t, err)
}

type testInformer struct {
	sri    *sharedResourceInformer
	client *dynamicclientset.ResourceClient
	dc     *fake.FakeDynamicClient
}

// testResourceMap returns one discovery map for all tests in this package.
// The fake discovery helper sleeps for one second, so do not call it in each
// test.
var testResourceMap = sync.OnceValue(func() *dynamicdiscovery.ResourceMap {
	simple := fakeclientset.NewClientset()
	fakeDiscovery := simple.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: testAPIVersion,
			APIResources: []metav1.APIResource{
				{
					Name:       testResource,
					Kind:       testKind,
					Namespaced: true,
					Group:      testGroup,
					Version:    testVersion,
					Verbs:      []string{"get", "list", "watch"},
				},
			},
		},
	}
	return discovery.NewFakeResourceMap(simple)
})

// newTestInformer starts a shared informer with a default resync period of
// one hour. So a short handler period must get its own timer.
func newTestInformer(t *testing.T, objects ...runtime.Object) *testInformer {
	t.Helper()

	ti := newUnstartedTestInformer(t, time.Hour, objects...)
	ti.start(t)
	return ti
}

// newUnstartedTestInformer creates a shared informer that does not run yet.
// Call start, or run the informer in the test.
func newUnstartedTestInformer(t *testing.T, defaultResync time.Duration, objects ...runtime.Object) *testInformer {
	t.Helper()

	dc := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		testGVR: testKind + "List",
	}, objects...)
	clientset := dynamicclientset.NewClientset(&rest.Config{}, testResourceMap(), dc)
	client, err := clientset.Resource(testAPIVersion, testResource)
	require.NoError(t, err, "can't create the resource client")

	return &testInformer{
		sri:    newSharedResourceInformer(client, defaultResync, func() {}),
		client: client,
		dc:     dc,
	}
}

// start runs the shared informer until the test ends and waits for its cache.
func (ti *testInformer) start(t *testing.T) {
	t.Helper()

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	go ti.sri.informer.Run(stopCh)
	require.True(t, cache.WaitForCacheSync(stopCh, ti.sri.informer.HasSynced), "shared informer cache did not sync")
}

// create adds a new object to the API that the informer watches.
func (ti *testInformer) create(t *testing.T, name string) {
	t.Helper()

	_, err := ti.client.Namespace(testNamespace).Create(context.Background(), newWidget(testNamespace, name, ""), metav1.CreateOptions{})
	require.NoError(t, err)
}

func newWidget(namespace, name string, ownerUID types.UID) *unstructured.Unstructured {
	widget := &unstructured.Unstructured{}
	widget.SetAPIVersion(testAPIVersion)
	widget.SetKind(testKind)
	widget.SetNamespace(namespace)
	widget.SetName(name)
	widget.SetUID(types.UID("uid-" + name))
	if ownerUID != "" {
		controller := true
		widget.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: testAPIVersion,
				Kind:       testKind,
				Name:       "parent",
				UID:        ownerUID,
				Controller: &controller,
			},
		})
	}
	return widget
}

// addBlockedHandler adds a handler that blocks in its add function. It
// returns after the handler is blocked. Call the returned function to release
// the handler.
func addBlockedHandler(t *testing.T, ti *testInformer, ri *ResourceInformer) func() {
	t.Helper()

	entered := make(chan struct{})
	unblock := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	_, err := ri.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(interface{}) {
			enterOnce.Do(func() { close(entered) })
			<-unblock
		},
	})
	require.NoError(t, err)
	ti.create(t, "blocking")
	waitForClose(t, entered, "the handler did not start")

	return func() { releaseOnce.Do(func() { close(unblock) }) }
}

// waitUntilRemovalTookHandlers waits until a removal call has taken the
// handlers from the wrapper. It reads private state only to put the test at
// the correct point in time.
func waitUntilRemovalTookHandlers(t *testing.T, ri *ResourceInformer) {
	t.Helper()

	require.Eventually(t, func() bool {
		ri.informerWrapper.mutex.Lock()
		defer ri.informerWrapper.mutex.Unlock()
		return len(ri.informerWrapper.handlers) == 0
	}, waitLimit, 5*time.Millisecond, "the removal did not start")
}

func waitForClose(t *testing.T, done <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(waitLimit):
		t.Fatal(message)
	}
}

// eventRecorder records the events that one handler gets.
type eventRecorder struct {
	mutex       sync.Mutex
	names       []string
	initial     []bool
	updateCount int
}

// handler returns a handler that records each add and each update.
func (r *eventRecorder) handler() cache.ResourceEventHandlerDetailedFuncs {
	return cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(obj interface{}, isInInitialList bool) {
			r.mutex.Lock()
			defer r.mutex.Unlock()
			r.names = append(r.names, obj.(*unstructured.Unstructured).GetName())
			r.initial = append(r.initial, isInInitialList)
		},
		UpdateFunc: func(_, _ interface{}) {
			r.mutex.Lock()
			defer r.mutex.Unlock()
			r.updateCount++
		},
	}
}

// add records an add without the initial list flag. Use it as the function
// that SkipInitialList wraps.
func (r *eventRecorder) add(obj interface{}) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.names = append(r.names, obj.(*unstructured.Unstructured).GetName())
}

func (r *eventRecorder) addNames() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return slices.Clone(r.names)
}

func (r *eventRecorder) initialFlags() []bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return slices.Clone(r.initial)
}

func (r *eventRecorder) updates() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.updateCount
}

// waitForAdd waits until the handler gets an add for the named object.
func (r *eventRecorder) waitForAdd(t *testing.T, name string) {
	t.Helper()

	if !assert.Eventually(t, func() bool {
		return slices.Contains(r.addNames(), name)
	}, waitLimit, 5*time.Millisecond) {
		t.Fatalf("the handler did not get an add for %q, it got adds for %v", name, r.addNames())
	}
}

// waitForUpdates waits until the handler has at least the given number of
// updates.
func (r *eventRecorder) waitForUpdates(t *testing.T, want int) {
	t.Helper()

	if !assert.Eventually(t, func() bool {
		return r.updates() >= want
	}, waitLimit, 5*time.Millisecond) {
		t.Fatalf("the handler got %d updates, want at least %d", r.updates(), want)
	}
}
