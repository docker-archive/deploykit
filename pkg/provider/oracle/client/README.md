# Oracle BMC Golang SDK

[![GoDoc](https://godoc.org/github.com/FrenchBen/oracle-sdk-go?status.svg)](https://godoc.org/github.com/FrenchBen/oracle-sdk-go)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/FrenchBen/oracle-sdk-go/master/LICENSE)
[![TravisCI](https://travis-ci.org/FrenchBen/oracle-sdk-go.svg?branch=master)](https://travis-ci.org/FrenchBen/oracle-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/FrenchBen/oracle-sdk-go)](https://goreportcard.com/report/github.com/FrenchBen/oracle-sdk-go)
[![Badge Badge](http://doyouevenbadge.com/github.com/FrenchBen/oracle-sdk-go)](http://doyouevenbadge.com)

Unofficial Oracle Bare Metal Cloud Golang SDK



<p align="center">
  <a href="http://golang.org" target="_blank"><img alt="Go package" src="https://golang.org/doc/gopher/pencil/gopherhat.jpg" width="20%" /></a>
</p>
<p align="center">
  <img src="https://cdn4.iconfinder.com/data/icons/linecon/512/add-128.png" alt="plus" />
</p>
<p align="center">
  <a href="https://www.oraclecloud.com/" target="_blank"><img src="https://static1.squarespace.com/static/587d94cf4402432706cdd02d/t/58cee1bd1b10e37dbd70b243/1489953226294/oracle-logo" alt="Oracle Cloud Logo"/></a>
</p>




## Requirements
The following variables are needed in order to make successful calls to the BMC API

* User - The user for the compartment
```
user=ocid1.user.oc1..aaaaaaaat5nvwcna5j6aqzjcaty5eqbb6qt2jvpkanghtgdaqedqw3rynjq
```
* Fingerprint - The API key fingerprint; this fingerprint can be found within the Oracle BMC console
```
fingerprint=20:3b:97:13:55:1c:5b:0d:d3:37:d8:50:4e:c5:3a:34
```
* KeyFile - The API key file location
```
key_file=~/.oraclebmc/bmcs_api_key.pem
```
* Tenancy - Your tenancy compartment ID
```
tenancy=ocid1.tenancy.oc1..aaaaaaaaba3pv6wkcr4jqae5f15p2b2m2yt2j6rx32uzr4h25vqstifsfdsq
```
* Region - The region corresponding to the compartment in use
```
region=us-ashburn-1
```

A default config file can also be used; the typical location for the Oracle BMC config is: `~/.oraclebmc/config`
The config file should have the following format:
```
[DEFAULT]
  user=ocid1.user.oc1..aaaaaaaat5nvwcna5j6aqzjcaty5eqbb6qt2jvpkanghtgdaqedqw3rynjq
  fingerprint=20:3b:97:13:55:1c:5b:0d:d3:37:d8:50:4e:c5:3a:34
  key_file=~/.oraclebmc/bmcs_api_key.pem
  tenancy=ocid1.tenancy.oc1..aaaaaaaaba3pv6wkcr4jqae5f15p2b2m2yt2j6rx32uzr4h25vqstifsfdsq
  region=us-ashburn-1
```

# Support Endpoint

The following endpoints are provided by the [Oracle BMC API](https://docs.us-phoenix-1.oraclecloud.com/api/)

| Name | Support |
| --- | --- |
| Audit API | NO |
| Database Service API | NO |
| Core Services API | YES-partial |
| Identity and Access Management Service API | NO |
| Load Balancing Service API | YES-partial |
| Object Storage Service API | NO |
| S3 Object Storage Service API | NO |

Only API supported at the moment is to get instance details
* List instances
* Get instance
* List VNIC
* Get VNIC
* List VNICAttachment
* Get VNICAttachment
* List SecurityList
* Get SecurityList
* Create SecurityList
* Update SecurityList
* Delete SecurityList
* List VirtualCloudNetwork
* Get VirtualCloudNetwork
* List Backend
* Get Backend
* Create Backend
* Delete Backend
* List BackendSet
* Get BackendSet
* Create BackendSet
* Delete BackendSet
* Create Listener
* Delete Listener
* List LoadBalancer
* Get LoadBalancer

