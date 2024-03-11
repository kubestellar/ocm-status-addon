package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/work/v1"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err.Error())
	}
	kubeconfig := flag.String("kubeconfig", filepath.Join(homeDir, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	namespace := flag.String("namespace", "cluster1", "(optional) namespace to use to generate the load")
	numObjects := flag.Int("num-objects", 10, "(optional) number of objects to create and delete")
	deleteOnly := flag.Bool("delete-only", false, "delete only and do not create any objects")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	workClient, err := workclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// delete and exit if delete-only
	if *deleteOnly {
		deleteAll(workClient, *namespace, *numObjects)
		fmt.Println("All deleted, program exits")
		os.Exit(0)
	}

	// now create, delete, and re-create the same objects consecutively
	createAll(workClient, *namespace, *numObjects)

	deleteAll(workClient, *namespace, *numObjects)

	createAll(workClient, *namespace, *numObjects)

}

func generatePodManifest(name, namespace, manifestNamespace string) *workv1.ManifestWork {
	pod := corev1.Pod{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busyb",
					Image:   "busybox:latest",
					Command: []string{"sleep", "inf"},
				},
			},
			NodeSelector: map[string]string{"node": "none"},
		},
	}

	podBytes, err := json.Marshal(pod)
	if err != nil {
		panic(err.Error())
	}

	rawExtension := runtime.RawExtension{
		Raw: podBytes,
	}

	manifestWork := &workv1.ManifestWork{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: manifestNamespace,
			Labels:    map[string]string{"managed-by.kubestellar.io/something": "true", "transport.kubestellar.io": "true"},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: rawExtension,
					},
				},
			},
		},
	}
	return manifestWork
}

func generateNameFromIndex(index int) string {
	return fmt.Sprintf("pod-%d", index)
}

func deleteAll(client *workclientset.Clientset, namespace string, numObjects int) {
	var err error
	for i := 0; i < numObjects; i++ {
		mName := generateNameFromIndex(i)
		err = client.WorkV1().ManifestWorks(namespace).Delete(context.Background(), mName, v1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			panic(err.Error())
		}
	}
}

func createAll(client *workclientset.Clientset, namespace string, numObjects int) {
	var err error
	for i := 0; i < numObjects; i++ {
		mName := generateNameFromIndex(i)
		manifest := generatePodManifest(mName, "default", namespace)
		_, err = client.WorkV1().ManifestWorks(namespace).Create(context.Background(), manifest, v1.CreateOptions{})
		if err != nil && apierrors.IsAlreadyExists(err) {
			wait.PollImmediateUntilWithContext(context.Background(), 500*time.Millisecond, func(ctx context.Context) (bool, error) {
				_, err = client.WorkV1().ManifestWorks(namespace).Create(context.Background(), manifest, v1.CreateOptions{})
				if err == nil {
					return true, nil
				}
				return false, nil
			})
		}
	}
}
