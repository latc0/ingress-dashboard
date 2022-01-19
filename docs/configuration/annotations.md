---
parent: Configuration
---

# Annotations

ingress-dashboard relies on annotations in
each [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) object to configure dashboard.

All annotations are optional.

## Description

Annotation: `ingress-dashboard/description`

Defines custom description for the ingress. If not defined, no description will be shown.

Example:

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/description: |
      This is demo service
spec:
  rules:
    - host: demo.example.com
      http:
        paths:
          - path: /foo/
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 8080
```

## Logo URL

Annotation: `ingress-dashboard/logo-url`

Defines custom logo URL for the ingress. It supports absolute URL (ex: `https://example.com/favicon.ico`) or relative
URL (ex: `/favicon.ico`). Relative URL should start from `/` and will be appended to the first endpoint in spec.

Example:

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/logo-url: "/favicon.ico"
spec:
  rules:
    - host: demo.example.com
      http:
        paths:
          - path: /foo/
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 8080
```

Logo URL will point to `http://demo.example.com/foo/favicon.ico`

If logo URL not defined, ingress-dashboard will try to detect it automatically:

* it will get root page for each defined endpoint, parse it as HTML and use `href` attribute as logo url for tags `link`
  with attribute `rel` equal to
    * `apple-touch-icon`
    * `shortcut icon`
    * `icon`
    * `alternate icon`
* in case no logo URL found in HTML, ingress-dashboard will check `<url>/favicon.ico` URL and in case of 200 code
  response will use it as logo-url

## Title

Annotation: `ingress-dashboard/title`

Defines custom service title. If not defined - ingress name will be used.

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/title: Demo App
spec:
  rules:
    - host: demo.example.com
      http:
        paths:
          - path: /foo/
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 8080
```

## Hide

Annotation: `ingress-dashboard/hide`

Accepts `true` or `false` (default) string value.

If it set to `true`, ingress-dashboard will not render it in UI and will skip logo-url detection logic.

## URL

Annotation: `ingress-dashboard/url`

Custom ingress URL. Could be used with load-balancers or reverse-proxies when public URL not the same as ingress. Also,
the provided URL will be used for TLS checks.

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/url: "https://example.com"
spec:
  rules:
    - host: demo.example.com
      http:
        paths:
          - path: /foo/
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 8080
```


## Assume TLS (force TLS)

Annotation: `ingress-dashboard/assume-tls`

Accepts `true` or `false` (string) value. Default is `false`.

If enabled, forcefully sets protocol to HTTPS. Could be useful in case of TLS termination on load-balancer or on
reverse-proxies.

```yaml
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo
  annotations:
    ingress-dashboard/assume-tls: "true"
spec:
  rules:
    - host: demo.example.com
      http:
        paths:
          - path: /foo/
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 8080
```
