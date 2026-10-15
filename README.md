# prism-go-client
![codecov.io](https://codecov.io/github/nutanix-cloud-native/prism-go-client/coverage.svg?branch=main)

Go client for Nutanix Prism APIs. Currently includes the following APIs:
- Nutanix v3 APIs for Prism Central
- Select Nutanix v4 APIs for Prism Central
- Konnector

## Fork development

The `ioplane` fork requires Go 1.27.0 and uses the pinned Go 1.27.2 Podman
image for every check. Earlier Go consumers must update their toolchain before
using this fork. Public SDK methods and module import paths are retained.

```sh
make -f Makefile.dev dev-build
make -f Makefile.dev check
```

The blocking checks cover module consistency, transport regressions, all offline
unit packages with race detection, vet/build and whole-SDK govulncheck.
The 28 existing Keploy fixtures requiring an external server are explicitly
excluded; offline fixtures do not establish live Prism compatibility.

## Contributing
See the [contributing docs](CONTRIBUTING.md).

## Support
### Community Plus

This code is developed in the open with input from the community through issues and PRs. A Nutanix engineering team serves as the maintainer. Documentation is available in the project repository. Troubleshooting support for Nutanix customers is available via the [Nutanix Support Portal](https://www.nutanix.com/support-services/product-support).

Issues and enhancement requests can be submitted in the [Issues tab of this repository](../../issues). Please search for and review the existing open issues before submitting a new issue.

## License
The project is released under version 2.0 of the [Apache license](http://www.apache.org/licenses/LICENSE-2.0).



