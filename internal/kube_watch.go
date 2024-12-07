package internal

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	v12 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const (
	AnnoDescription = "ingress-dashboard/description"
	AnnoLogoURL     = "ingress-dashboard/logo-url"
	AnnoTitle       = "ingress-dashboard/title"
	AnnoHide        = "ingress-dashboard/hide"       // do not display ingress in dashboard
	AnnoURL         = "ingress-dashboard/url"        // custom ingress URL (could be used with load-balancers or reverse-proxies)
	AnnoAssumeTLS   = "ingress-dashboard/assume-tls" // force protocol as HTTPS (for SSL termination on load-balancers)
	syncInterval    = 30 * time.Second
	tlsInterval     = time.Hour
)

type Receiver interface {
	Set(ingresses []Ingress)
}

func WatchKubernetes(global context.Context, clientset kubernetes.Interface, receiver Receiver, fetchHostInfo bool) {
	ctx, cancel := context.WithCancel(global)
	defer cancel()

	watcher := newWatcher(ctx, receiver, clientset)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		watcher.runWatcher(ctx, clientset)
	}()

	if fetchHostInfo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			watcher.runLogoFetcher(ctx)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			watcher.runCertsInfoCheck(ctx)
		}()
	}
	wg.Wait()
}

func newWatcher(global context.Context, receiver Receiver, clientset kubernetes.Interface) *kubeWatcher {
	return &kubeWatcher{
		global:     global,
		cache:      make(map[string]Ingress),
		receiver:   receiver,
		checkLogos: make(chan struct{}, 1),
		checkCerts: make(chan struct{}, 1),
		clientset:  clientset,
	}
}

type kubeWatcher struct {
	global     context.Context
	clientset  kubernetes.Interface
	cache      map[string]Ingress
	lock       sync.RWMutex
	receiver   Receiver
	checkLogos chan struct{}
	checkCerts chan struct{}
}

func (kw *kubeWatcher) OnAdd(obj interface{}) {
	kw.upsertIngress(kw.global, obj)
}

func (kw *kubeWatcher) OnUpdate(_, newObj interface{}) {
	kw.upsertIngress(kw.global, newObj)
}

func (kw *kubeWatcher) OnDelete(obj interface{}) {
	defer kw.notify()

	kw.lock.Lock()
	defer kw.lock.Unlock()
	ing, ok := obj.(*v12.IngressClass)
	if !ok {
		return
	}
	delete(kw.cache, string(ing.UID))
}

func (kw *kubeWatcher) runLogoFetcher(ctx context.Context) {
	for {
		for _, ing := range kw.items() {
			if !ing.Hide && ing.LogoURL == "" && len(ing.Refs) > 0 {
				ing.LogoURL = detectIconURL(ctx, ing.Refs[0].URL)
				if ing.LogoURL != "" {
					kw.updateLogo(ing)
				}
			}
		}
		kw.receiver.Set(kw.items())
		select {
		case <-ctx.Done():
			return
		case <-kw.checkLogos:
		}
	}
}

func (kw *kubeWatcher) runWatcher(ctx context.Context, clientset kubernetes.Interface) {
	informerFactory := informers.NewSharedInformerFactory(clientset, syncInterval)
	informer := informerFactory.Networking().V1().Ingresses().Informer()

	informer.AddEventHandler(kw)
	informer.Run(ctx.Done())
}

func (kw *kubeWatcher) upsertIngress(ctx context.Context, obj interface{}) {
	defer kw.notify()
	ing, ok := obj.(*v12.Ingress)
	if !ok {
		return
	}
	ingress := kw.inspectIngress(ctx, ing)

	kw.lock.Lock()
	defer kw.lock.Unlock()

	if oldIngress, exists := kw.cache[ingress.UID]; exists {
		// preserve discovered logo
		oldLogoURL := oldIngress.LogoURL
		if oldLogoURL != "" && ingress.LogoURL == "" {
			ingress.LogoURL = oldLogoURL
		}

		// preserve cert info as initial value
		if !oldIngress.Cert.Expiration.IsZero() && ingress.Cert.Expiration.IsZero() {
			ingress.Cert = oldIngress.Cert
		}
	}

	kw.cache[ingress.UID] = ingress
}

func (kw *kubeWatcher) notify() {
	kw.receiver.Set(kw.items())
	select {
	case kw.checkLogos <- struct{}{}:
	default:
	}

	select {
	case kw.checkCerts <- struct{}{}:
	default:
	}
}

func (kw *kubeWatcher) items() []Ingress {
	kw.lock.RLock()
	defer kw.lock.RUnlock()

	return toList(kw.cache)
}

func (kw *kubeWatcher) updateLogo(ingress Ingress) {
	kw.lock.Lock()
	defer kw.lock.Unlock()
	old, exists := kw.cache[ingress.UID]
	if !exists || old.LogoURL != "" {
		return
	}
	old.LogoURL = ingress.LogoURL
	kw.cache[ingress.UID] = old
}

func (kw *kubeWatcher) updateCertInfo(ingress Ingress) {
	kw.lock.Lock()
	defer kw.lock.Unlock()
	old, exists := kw.cache[ingress.UID]
	if !exists {
		return
	}
	old.Cert = ingress.Cert
	kw.cache[ingress.UID] = old
}

func (kw *kubeWatcher) inspectIngress(ctx context.Context, ing *v12.Ingress) Ingress {
	forceTLS := toBool(ing.Annotations[AnnoAssumeTLS], false)

	return Ingress{
		Class:       getClassName(ing),
		Name:        ing.Name,
		Namespace:   ing.Namespace,
		Title:       ing.Annotations[AnnoTitle],
		ID:          ing.Namespace + "." + ing.Name,
		UID:         string(ing.UID),
		Description: ing.Annotations[AnnoDescription],
		LogoURL:     ing.Annotations[AnnoLogoURL],
		Hide:        toBool(ing.Annotations[AnnoHide], false),
		Refs:        kw.getRefs(ctx, ing, forceTLS),
		TLS:         forceTLS || len(ing.Spec.TLS) > 0,
	}
}

func toList(cache map[string]Ingress) []Ingress {
	var cp = make([]Ingress, 0, len(cache))
	for _, ing := range cache {
		cp = append(cp, ing)
	}
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].ID < cp[j].ID
	})

	return cp
}

func (kw *kubeWatcher) getRefs(ctx context.Context, ing *v12.Ingress, forceTLS bool) []Ref {
	if staticURL, ok := ing.Annotations[AnnoURL]; ok {
		podsNum, err := kw.getTotalPodsNum(ctx, ing)
		if err != nil {
			log.Println("failed count pods:", err)
		}

		return []Ref{{
			URL:  staticURL,
			Pods: podsNum,
		}}
	}

	proto := "http://"
	if forceTLS || len(ing.Spec.TLS) > 0 {
		proto = "https://"
	}

	var refs []Ref
	baseURL := proto + ing.Status.LoadBalancer.Ingress[0].Hostname
	for _, rule := range ing.Spec.Rules {
		// baseURL := proto + rule.Host
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				var ref = Ref{
					URL: baseURL + path.Path,
				}
				numPods, err := kw.getPodsNum(ctx, ing.Namespace, path.Backend.Service)
				if err != nil {
					log.Println("failed to get pods num for ingress", ing.Name, "in", ing.Namespace, "for path", path.Path, "-", err)
				} else {
					ref.Pods = numPods
				}
				refs = append(refs, ref)
			}
		}
	}

	return refs
}

func (kw *kubeWatcher) getTotalPodsNum(ctx context.Context, ing *v12.Ingress) (int, error) {
	var sum int
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			numPods, err := kw.getPodsNum(ctx, ing.Namespace, path.Backend.Service)
			if err != nil {
				return sum, fmt.Errorf("get pods num for ingress %s in %s for path %s: %w", ing.Name, ing.Namespace, path.Path, err)
			}
			sum += numPods
		}
	}

	return sum, nil
}

func (kw *kubeWatcher) getPodsNum(ctx context.Context, namespace string, svc *v12.IngressServiceBackend) (int, error) {
	if svc == nil {
		return 0, nil
	}
	info, err := kw.clientset.CoreV1().Services(namespace).Get(ctx, svc.Name, v1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("get service %s in %s: %w", svc.Name, namespace, err)
	}

	var extHosts = len(info.Spec.ExternalIPs)
	if extHosts == 0 && info.Spec.ExternalName != "" {
		// reference by DNS to external host
		extHosts = 1
	}

	return len(info.Spec.ClusterIPs) + extHosts, nil
}

func (kw *kubeWatcher) runCertsInfoCheck(ctx context.Context) {
	timer := time.NewTicker(tlsInterval)
	defer timer.Stop()

	for {
		kw.scanTLSCerts(ctx)
		select {
		case <-kw.checkCerts:
		case <-timer.C:
		case <-ctx.Done():
			return
		}
	}
}

func (kw *kubeWatcher) scanTLSCerts(ctx context.Context) {
	for _, item := range kw.items() {
		if !item.TLS {
			continue
		}

		info := fetchCertInfo(ctx, item)
		if info == nil {
			log.Println("no cert info for", item.ID)

			continue
		}

		item.Cert = *info
		kw.updateCertInfo(item)
		kw.receiver.Set(kw.items())
	}
	kw.receiver.Set(kw.items())
}

func fetchCertInfo(ctx context.Context, item Ingress) *CertInfo {
	for _, u := range item.Refs {
		if parsedURL, err := url.Parse(u.URL); err == nil {
			host := parsedURL.Hostname()
			crtInfo, err := Expiration(ctx, host)
			if err != nil {
				log.Println("failed get expiration time", host, ":", err)

				continue
			}

			return &crtInfo // stop on first ref
		}
	}

	return nil
}

func toBool(value string, defaultValue bool) bool {
	if v, err := strconv.ParseBool(value); err == nil {
		return v
	}

	return defaultValue
}

func getClassName(ing *v12.Ingress) string {
	const anno = "kubernetes.io/ingress.class"
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName
	}

	return ing.Annotations[anno]
}
