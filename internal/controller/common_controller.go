package controller

const (
	// daemonPort defines the port for the Chia daemon
	daemonPort = 55400

	// chiaExporterPort defines the port for Chia Exporter instances
	chiaExporterPort = 9914
)

// controllerOwner tells k8s objects that the CR that created it is its controller owner
var controllerOwner = true
