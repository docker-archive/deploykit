

vagrant up

export TOKEN=`curl -sk -H "Content-Type: application/json" -X POST https://:9093/login -d '{"username": "admin", "password": "admin123"}' | jq '.["token"]' | sed 's|"||g'`

echo "Create VirtualBox SKU in RackHD"

curl -sk -H "Authorization: JWT $TOKEN" -H "Content-Type: application/json" -X POST https://:9093/api/current/skus -d '{"name": "VirtualBox", "rules": [{"path": "dmi.Base Board Information.Product Name", "equals": "VirtualBox"}]}'
