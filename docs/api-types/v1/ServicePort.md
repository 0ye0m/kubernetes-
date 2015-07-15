###ServicePort###

---
* name: 
  * **_type_**: string
  * **_description_**: the name of this port; optional if only one port is defined
* nodePort: 
  * **_type_**: integer
  * **_description_**: the port on each node on which this service is exposed when type=NodePort or LoadBalancer; usually assigned by the system; if specified, it will be allocated to the service if unused or else creation of the service will fail; see http://releases.k8s.io/HEAD/docs/services.md#type--nodeport
* port: 
  * **_type_**: integer
  * **_description_**: the port number that is exposed
* protocol: 
  * **_type_**: string
  * **_description_**: the protocol used by this port; must be UDP or TCP; TCP if unspecified
* targetPort: 
  * **_type_**: string
  * **_description_**: number or name of the port to access on the pods targeted by the service; defaults to the service port; number must be in the range 1 to 65535; name must be an IANA_SVC_NAME; see http://releases.k8s.io/HEAD/docs/services.md#defining-a-service
