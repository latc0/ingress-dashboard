![k-id](https://user-images.githubusercontent.com/6597086/145367873-ae85fba7-d3aa-47ba-8100-1ce6518aa463.png)

Automatic dashboard generation for Ingress objects.

Features:

* No JS
* Supports OIDC (Keycloak, Google, Okta, ...) and Basic authorization
* Automatic discovery of Ingress objects, configurable by annotations
* Supports static configuration (in addition to Ingress objects)
* Multiarch docker images: for amd64 and for arm64
* Automatic even-based updates

Limitations:

* Supports only v1/Ingress kind.
* Doesn't support Ingress Reference kind, only Service type
* Hosts number per Ingress calculated each Ingress update or after refresh (30s by default)

<img alt="image" src="https://user-images.githubusercontent.com/6597086/145249365-52035d08-469d-460e-b42c-e6af5d271e10.png">
