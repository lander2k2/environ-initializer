package environ

import (
	"k8s.io/client-go/tools/cache"
)

type environInitializer interface {
	getResourceController() cache.Controller
}

func Initialize(e environInitializer) cache.Controller {

	return e.getResourceController()

}
