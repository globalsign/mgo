package mgo

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
)

func ExampleCredential_x509Authentication() {
	// MongoDB follows RFC2253 for the ordering of the DN - if the order is
	// incorrect when creating the user in Mongo, the client will not be able to
	// connect.
	//
	// The best way to generate the DN with the correct ordering is with
	// openssl:
	//
	// 		openssl x509 -in client.crt -inform PEM -noout -subject -nameopt RFC2253
	// 		subject= CN=Example App,OU=MongoDB Client Authentication,O=GlobalSign,C=GB
	//
	//
	// And then create the user in MongoDB with the above DN:
	//
	//		db.getSiblingDB("$external").runCommand({
	//			createUser: "CN=Example App,OU=MongoDB Client Authentication,O=GlobalSign,C=GB",
	//			roles: [
	//				{ role: 'readWrite', db: 'bananas' },
	//				{ role: 'userAdminAnyDatabase', db: 'admin' }
	//			],
	//			writeConcern: { w: "majority" , wtimeout: 5000 }
	//		})
	//
	//
	// References:
	// 		- https://docs.mongodb.com/manual/tutorial/configure-x509-client-authentication/
	// 		- https://docs.mongodb.com/manual/core/security-x.509/
	//

	// Read in the PEM encoded X509 certificate.
	//
	// See the client.pem file at the path below.
	clientCertPEM, err := ioutil.ReadFile("harness/certs/client.pem")

	// Read in the PEM encoded private key.
	clientKeyPEM, err := ioutil.ReadFile("harness/certs/client.key")

	// Parse the private key, and the public key contained within the
	// certificate.
	clientCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)

	// Parse the actual certificate data
	clientCert.Leaf, err = x509.ParseCertificate(clientCert.Certificate[0])

	// Use the cert to set up a TLS connection to Mongo
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},

		// This is set to true so the example works within the test
		// environment.
		//
		// DO NOT set InsecureSkipVerify to true in a production
		// environment - if you use an untrusted CA/have your own, load
		// its certificate into the RootCAs value instead.
		//
		// RootCAs: myCAChain,
		InsecureSkipVerify: true,
	}

	// Connect to Mongo using TLS
	host := "localhost:40003"
	session, err := DialWithInfo(&DialInfo{
		Addrs: []string{host},
		DialServer: func(addr *ServerAddr) (net.Conn, error) {
			return tls.Dial("tcp", host, tlsConfig)
		},
	})

	// Authenticate using the certificate
	cred := &Credential{Certificate: tlsConfig.Certificates[0].Leaf}
	if err := session.Login(cred); err != nil {
		panic(err)
	}

	// Done! Use mgo as normal from here.
	//
	// You should actually check the error code at each step.
	_ = err
}
