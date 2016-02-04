package etcd

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/registry/cluster"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
)

type REST struct {
	*etcdgeneric.Etcd
}

type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return &api.Cluster{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx api.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}

// NewREST returns a RESTStorage object that will work against clusters.
func NewREST(s storage.Interface, storageDecorator generic.StorageDecorator) (*REST, *StatusREST) {
	prefix := "/clusters"

	newListFunc := func() runtime.Object { return &api.ClusterList{} }
	storageInterface := storageDecorator(
		s, 100, &api.Cluster{}, prefix, false, newListFunc)

	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.Cluster{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx api.Context) string {
			return prefix
		},
		KeyFunc: func(ctx api.Context, name string) (string, error) {
			return etcdgeneric.NoNamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Cluster).Name, nil
		},
		PredicateFunc: cluster.MatchCluster,
		QualifiedResource: api.Resource("clusters"),

		CreateStrategy: cluster.Strategy,
		UpdateStrategy: cluster.Strategy,

		ReturnDeletedObject: true,

		Storage: storageInterface,
	}

	statusStore := *store
	statusStore.UpdateStrategy = cluster.StatusStrategy


	return &REST{store}, &StatusREST{store: &statusStore}
}
