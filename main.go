package main

import (
	"encoding/json"
	"flag"
	"github.com/julienschmidt/httprouter"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"net/http"
	"path/filepath"
	"strings"
)

type ClusterConnection struct {
	connections map[string]*kubernetes.Clientset
}

func NewClusterConnection() *ClusterConnection {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	cfg, err := clientcmd.LoadFromFile(*kubeconfig)
	if err != nil {
			panic(err.Error())
	}

	clusterConnections := make(map[string]*kubernetes.Clientset)

	for _, context := range cfg.Contexts {
		// TODO: respect to name differences between contexts-clusters
		cc := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{CurrentContext: context.Cluster})

		restConfig, err := cc.ClientConfig()
		if err != nil {
			panic(err.Error())
		}

		clientSet, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			panic(err.Error())
		}

		clusterConnections[context.Cluster] = clientSet
	}

	return &ClusterConnection{
		connections: clusterConnections,
	}
}

func (c ClusterConnection) GetDeployments(clusters []string) (map[string][]v1.Deployment, []error) {
	deploymentClusterMap := make(map[string][]v1.Deployment)

	//TODO return correct errors for each cluster
	var errors []error

	for _, cluster := range clusters {
		deployments, err := c.connections[cluster].AppsV1().Deployments("").List(metav1.ListOptions{})
		if err != nil {
			errors = append(errors, err)
		}

		deploymentClusterMap[cluster] = deployments.Items
	}

	return deploymentClusterMap, errors
}

var clusterConnection *ClusterConnection

func main() {
	clusterConnection = NewClusterConnection()
	/*
		//PUT clusters/add
		//GET clusters/

		//GET resources/deployments?clusters=
		//PUT resources/deployments
	*/

	router := httprouter.New()
	router.GET("/resources/deployments", GetDeployments)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

func GetDeployments(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	clusters := r.URL.Query().Get("clusters")

	deployments, _ := clusterConnection.GetDeployments(strings.Split(clusters, ","))

	responseBytes, _ := json.Marshal(deployments)

	w.Write(responseBytes)
	w.WriteHeader(http.StatusOK)
}
