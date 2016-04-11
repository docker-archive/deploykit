##############################################################################
#
# Deploy the compiled binary to another git repo
#

DEPLOY_REPO_URL:=git@github.com:infradash/public.git
DEPLOY_REPO_BRANCH:=gh-pages
DEPLOY_LOCAL_REPO:=build/deploy
DEPLOY_USER_EMAIL:=deploy@infradash.com
DEPLOY_USER_NAME:=deploy
DEPLOY_DIR:=redpill/latest

deploy-git-checkout:
	mkdir -p ./build/deploy
	git clone $(DEPLOY_REPO_URL) $(DEPLOY_LOCAL_REPO)
	cd $(DEPLOY_LOCAL_REPO) && git config --global user.email $(DEPLOY_USER_EMAIL) && git config --global user.name $(DEPLOY_USER_NAME) && git checkout $(DEPLOY_REPO_BRANCH)

deploy-git: deploy-git-checkout
	mkdir -p $(DEPLOY_LOCAL_REPO)/$(DEPLOY_DIR) && cp -r $(BUILD_DIR) $(DEPLOY_LOCAL_REPO)/$(DEPLOY_DIR) && echo $(DOCKER_IMAGE) > $(DEPLOY_LOCAL_REPO)/$(DEPLOY_DIR)/DOCKER 
	cd $(DEPLOY_LOCAL_REPO) && git add -v $(DEPLOY_DIR) && git commit -m "Version $(GIT_TAG) Commit $(GIT_COMMIT_HASH) Build $(CIRCLE_BUILD_NUM)" -a && git push

