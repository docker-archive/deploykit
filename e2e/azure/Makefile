all: test-azure

test-azure:
# Values taken from Docker Machine
	AZURE_CLIENT_ID="637ddaba-219b-43b8-bf19-8cea500cf273" \
	AZURE_SUBSCRIPTION_ID="71be52f3-def6-4c7c-85b4-c1053ba503b2" \
	AZURE_ENVIRONMENT="AzurePublicCloud" \
	${GODEP} go test ./... -v ${TEST_ARGS}
