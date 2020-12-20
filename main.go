package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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

	// TODO: handle error
	cfg, _ := clientcmd.LoadFromFile(*kubeconfig)

	clusterConnections := make(map[string]*kubernetes.Clientset)

	for _, context := range cfg.Contexts {
		// TODO: respect to name differences between contexts-clusters
		cc := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{CurrentContext: context.Cluster})
		// TODO: handle error
		restConfig, _ := cc.ClientConfig()

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

	var errors []error

	for _, cluster := range clusters {
		deployments, err := c.connections[cluster].AppsV1().Deployments("").List(metav1.ListOptions{})
		if err != nil {
			errors = append(errors, err)
		} else {
			errors = append(errors, nil)
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

type GetDeploymentsResponse struct {
	ClusterName string          `json:"clusterName"`
	Deployments []v1.Deployment `json:"deployments"`
	Error       error           `json:"error"`
}

func GetDeployments(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	clusters := r.URL.Query().Get("clusters")
	clusterList := strings.Split(clusters, ",")

	deploymentClusterMap := make([]GetDeploymentsResponse, len(clusterList))

	deployments, errors := clusterConnection.GetDeployments(clusterList)

	for index, cluster := range clusterList {
		deploymentClusterMap[index].ClusterName = cluster
		if errors[index] != nil {
			deploymentClusterMap[index].Error = errors[index]
		} else {
			deploymentClusterMap[index].Deployments = deployments[cluster]
		}
	}

	responseBytes, err := json.Marshal(deploymentClusterMap)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(responseBytes)
		w.WriteHeader(http.StatusOK)
	}
}
