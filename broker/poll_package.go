package broker

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/lager"
)

func (broker *SCSBroker) pollPackage(pkg ccv3.Package) (ccv3.Package, ccv3.Warnings, error) {
	var allWarnings ccv3.Warnings
	cfClient, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("broker.PollPackage: broker.GetClient()", err)
		return ccv3.Package{}, nil, errors.New("broker.PollPackage: Couldn't start session: " + err.Error())
	}

	var pkgCache ccv3.Package

	for pkg.State != constant.PackageReady && pkg.State != constant.PackageFailed && pkg.State != constant.PackageExpired {
		time.Sleep(1000000000)
		ccPkg, warnings, err := cfClient.GetPackage(pkg.GUID)
		broker.Logger.Info("broker.PollPackage: polling package state", lager.Data{
			"package_guid": pkg.GUID,
			"state":        pkg.State,
		})

		broker.showWarnings(warnings, ccPkg)

		allWarnings = append(allWarnings, warnings...)
		if err != nil {
			broker.Logger.Error("broker.PollPackage: cfClient.GetPackage()", err)
			return ccv3.Package{}, allWarnings, err
		}
		pkgCache = pkg
		pkg = ccv3.Package(ccPkg)
	}

	broker.Logger.Info("polling package final state:", lager.Data{
		"package_guid": pkg.GUID,
		"state":        pkg.State,
	})

	if pkg.State == constant.PackageFailed {
		err := errors.New("Package Failed")
		broker.Logger.Error(fmt.Sprintf("Service Package Error: Package State %s", pkg.State), err, lager.Data{"Original Package": pkgCache, "Checked Package": pkg})
		return ccv3.Package{}, allWarnings, err
	} else if pkg.State == constant.PackageExpired {
		err := errors.New("Package Expired")
		broker.Logger.Error(fmt.Sprintf("Service Package Error: Package State %s", pkg.State), err, lager.Data{"Original Package": pkgCache, "Checked Package": pkg})
		return ccv3.Package{}, allWarnings, err
	}

	return pkg, allWarnings, nil
}
