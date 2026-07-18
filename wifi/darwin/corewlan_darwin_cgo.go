//go:build darwin && cgo

package darwin

/*
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation
#include <stdlib.h>
#include "corewlan_bridge.h"
*/
import "C"

import (
	"unsafe"
)

func scanVisibleNetworks(device string) ([]scannedNetwork, error) {
	cDevice := C.CString(device)
	defer C.free(unsafe.Pointer(cDevice))

	var output *C.char
	var errorMessage *C.char
	status := C.wifitui_corewlan_scan(cDevice, &output, &errorMessage)
	if output != nil {
		defer C.wifitui_corewlan_free(output)
	}
	if errorMessage != nil {
		defer C.wifitui_corewlan_free(errorMessage)
	}

	if status != coreWLANStatusSuccess {
		message := "CoreWLAN scan failed"
		if errorMessage != nil {
			message = C.GoString(errorMessage)
		}
		return nil, coreWLANStatusError(int(status), message)
	}
	if output == nil {
		return nil, coreWLANStatusError(coreWLANStatusProtocol, "CoreWLAN returned an empty response")
	}
	return decodeCoreWLANScan([]byte(C.GoString(output)))
}
