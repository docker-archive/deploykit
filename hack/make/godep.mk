godep-save:
	echo "Running godep"
	godep save ./main/... ./pkg/...
	git status