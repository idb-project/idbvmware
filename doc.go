/*Command idbvmware querys virtual machine information and adds the
machines found to a IDB instance.
It is supposed to be run regularly by cron.

Command line arguments

The following command line arguments are supported:
  -config string
    	config file (default "/etc/bytemine/idbvmware.json")
  -example
    	write an example config to idbvmware.json.example in the current dir.
  -version
    	display version and exit
  -dryrun
  		don't edit the IDB


Configuration file

The configuration is a JSON file with the following fields:
	Create: Create machines if they don't exists in the IDB
	IdbUrl: IDB API URL, eg. https://idb.example.com
	IdbToken: IDB API Token
	VmwareUrl: VMware API URL, eg. https://root:password@localhost:443/sdk
	Lookup: Try to do a reverse lookup for machines with invalid or unknown fqdn
	UnknownSuffix: Suffix for machines with invalid or unknown fqdn
	InsecureSkipVerify: Ignore invalid SSL chains
	Debug: Debug mode

*/
package main
