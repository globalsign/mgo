Contributing
-------------------------

We really appreciate contributions, but they must meet the following requirements:

* A PR should have a brief description of the problem/feature being proposed
* Pull requests should target the `development` branch
* Existing tests should pass and any new code should be covered with it's own test(s) (use [travis-ci](https://travis-ci.org))
* New functions should be [documented](https://blog.golang.org/godoc-documenting-go-code) clearly
* Code should pass `golint`, `go vet` and `go fmt`

We merge PRs into `development`, which is then tested in a sharded, replicated environment in our datacenter for regressions. Once everyone is happy, we merge to master - this is to maintain a bit of quality control past the usual PR process.

**Thanks** for helping!

# How to test the code
In order to run the tests, you need the following installed (assuming Ubuntu)

* daemontools (for svstat)
* mongo (for mongo client)
* mongodb (for mongo server)

Before running the tests, you need to start the test mongo server with `make startdb`. After the tests are done, you
can tear it down with `make stopdb`.

The tests to run are defined in `.travis.yml` under the *script* section.

## Note about DNS Lookup of SRV records
If you are testing on Linux, you may run into an error that net.LookupSRV cannot parse the DNS response.
In this case, you are likely using a distribution that uses systemd for DNS and go does not handle it yet.
The easiest work around is to change your /etc/resolv.conf file and set a remote nameserver; 8.8.8.8 or 9.9.9.9.