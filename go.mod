module github.com/cloudfoundry-community/scs-broker

go 1.22

replace github.com/docker/docker => github.com/docker/engine v17.12.0-ce-rc1.0.20200531234253-77e06fda0c94+incompatible

replace github.com/SermoDigital/jose => github.com/SermoDigital/jose v0.9.2-0.20161205224733-f6df55f235c2

replace github.com/mailru/easyjson => github.com/mailru/easyjson v0.0.0-20180323154445-8b799c424f57

replace github.com/cloudfoundry/sonde-go => github.com/cloudfoundry/sonde-go v0.0.0-20171206171820-b33733203bb4

replace code.cloudfoundry.org/go-log-cache => code.cloudfoundry.org/go-log-cache v1.0.1-0.20200316170138-f466e0302c34

require (
	code.cloudfoundry.org/cli v7.1.0+incompatible
	code.cloudfoundry.org/lager v2.0.0+incompatible
	github.com/cloudfoundry-community/go-cf-clients-helper v1.0.1
	github.com/cloudfoundry-community/go-cfclient/v2 v2.0.0
	github.com/cloudfoundry-community/go-uaa v0.3.1
	github.com/cloudfoundry-community/spring-cloud-services-cli-config-parser v1.0.3
	github.com/pivotal-cf/brokerapi v6.4.2+incompatible
	gopkg.in/yaml.v3 v3.0.1
)

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20200131002437-cf55d5288a48 // indirect
	code.cloudfoundry.org/cfnetworking-cli-api v0.0.0-20190103195135-4b04f26287a6 // indirect
	code.cloudfoundry.org/jsonry v1.1.4 // indirect
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/SermoDigital/jose v0.9.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/bmatcuk/doublestar v1.3.1 // indirect
	github.com/bmizerany/pat v0.0.0-20170815010413-6226ea591a40 // indirect
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1 // indirect
	github.com/cloudfoundry/bosh-cli v6.2.1+incompatible // indirect
	github.com/cloudfoundry/bosh-utils v0.0.0-20200606100138-7f673ba6be2a // indirect
	github.com/cloudfoundry/noaa v2.1.0+incompatible // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20200416163440-a42463ba266b // indirect
	github.com/cppforlife/go-patch v0.2.0 // indirect
	github.com/drewolson/testflight v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.0.0 // indirect
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jessevdk/go-flags v0.0.0-20170926144705-f88afde2fa19 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/tedsuo/rata v1.0.0 // indirect
	golang.org/x/crypto v0.25.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/term v0.22.0 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.28 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
