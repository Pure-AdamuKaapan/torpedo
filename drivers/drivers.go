package drivers

const (
	// ProviderAws for aws provider
	ProviderAws = "aws"
	// ProviderAzure for azure provider
	ProviderAzure = "azure"
	// ProviderGke for gke provider
	ProviderGke = "gke"
	// ProviderPortworx for portworx provider
	ProviderPortworx = "pxd"
	// ProviderNfs for nfs provider
	ProviderNfs = "nfs"
)

// Driver specifies the most basic methods to be implemented by a Torpedo driver.
type Driver interface {
	// Init the driver.
	Init() error
}
