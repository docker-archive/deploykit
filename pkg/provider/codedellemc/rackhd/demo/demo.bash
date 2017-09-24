vagrant up

if [ ! -f ./CentOS-7-x86_64-DVD-1611.iso ]
then
    wget http://mirrors.usc.edu/pub/linux/distributions/centos/7/isos/x86_64/CentOS-7-x86_64-DVD-1611.iso
fi

export TOKEN=`curl -sk -H "Content-Type: application/json" -X POST -d '{"username": "admin", "password": "admin123"}' https://localhost:9093/login | jq '.["token"]' | sed 's|"||g'`

echo "Create VirtualBox SKU in RackHD"

curl -sk -H "Authorization: JWT $TOKEN" -H "Content-Type: application/json" -X POST -d '{ "name": "VirtualBox", "rules": [ { "path": "dmi.Base Board Information.Product Name", "equals": "VirtualBox" } ] }' https://localhost:9093/api/current/skus
