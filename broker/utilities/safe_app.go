package utilities

import "code.cloudfoundry.org/cli/resources"

func SafeApp(app resources.Application) resources.Application {
	return resources.Application{
		GUID:                app.GUID,
		StackName:           app.StackName,
		LifecycleBuildpacks: app.LifecycleBuildpacks,
		LifecycleType:       app.LifecycleType,
		Metadata:            app.Metadata,
		Name:                app.Name,
		State:               app.State,
	}
}
