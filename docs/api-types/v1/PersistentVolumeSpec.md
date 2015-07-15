###PersistentVolumeSpec###

---
* accessModes: 
  * **_type_**: [][PersistentVolumeAccessMode](PersistentVolumeAccessMode.md)
  * **_description_**: all ways the volume can be mounted; see http://releases.k8s.io/HEAD/docs/persistent-volumes.md#access-modes
* awsElasticBlockStore: 
  * **_type_**: [AWSElasticBlockStoreVolumeSource](AWSElasticBlockStoreVolumeSource.md)
  * **_description_**: AWS disk resource provisioned by an admin; see http://releases.k8s.io/HEAD/docs/volumes.md#awselasticblockstore
* capacity: 
  * **_type_**: any
  * **_description_**: a description of the persistent volume's resources and capacityr; see http://releases.k8s.io/HEAD/docs/persistent-volumes.md#capacity
* claimRef: 
  * **_type_**: [ObjectReference](ObjectReference.md)
  * **_description_**: when bound, a reference to the bound claim; see http://releases.k8s.io/HEAD/docs/persistent-volumes.md#binding
* gcePersistentDisk: 
  * **_type_**: [GCEPersistentDiskVolumeSource](GCEPersistentDiskVolumeSource.md)
  * **_description_**: GCE disk resource provisioned by an admin; see http://releases.k8s.io/HEAD/docs/volumes.md#gcepersistentdisk
* glusterfs: 
  * **_type_**: [GlusterfsVolumeSource](GlusterfsVolumeSource.md)
  * **_description_**: Glusterfs volume resource provisioned by an admin; see http://releases.k8s.io/HEAD/examples/glusterfs/README.md
* hostPath: 
  * **_type_**: [HostPathVolumeSource](HostPathVolumeSource.md)
  * **_description_**: a HostPath provisioned by a developer or tester; for develment use only; see http://releases.k8s.io/HEAD/docs/volumes.md#hostpath
* iscsi: 
  * **_type_**: [ISCSIVolumeSource](ISCSIVolumeSource.md)
  * **_description_**: an iSCSI disk resource provisioned by an admin
* nfs: 
  * **_type_**: [NFSVolumeSource](NFSVolumeSource.md)
  * **_description_**: NFS volume resource provisioned by an admin; see http://releases.k8s.io/HEAD/docs/volumes.md#nfs
* persistentVolumeReclaimPolicy: 
  * **_type_**: string
  * **_description_**: what happens to a volume when released from its claim; Valid options are Retain (default) and Recycle.  Recyling must be supported by the volume plugin underlying this persistent volume. See http://releases.k8s.io/HEAD/docs/persistent-volumes.md#recycling-policy
* rbd: 
  * **_type_**: [RBDVolumeSource](RBDVolumeSource.md)
  * **_description_**: rados block volume that will be mounted on the host machine; see http://releases.k8s.io/HEAD/examples/rbd/README.md
