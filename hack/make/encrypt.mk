#############################################################################
#
# Targets for encryption / decryption of files
#

# 'private' task for echoing instructions
encrypt-pwd-prompt: encrypt-mk_dirs

# Make directories based the file paths
encrypt-mk_dirs:
	@mkdir -p encrypt decrypt ;


# Decrypt files in the encrypt/ directory
decrypt: encrypt-pwd-prompt
	@echo "Decrypt the files in a given directory (those with .cast5 extension)."
	@read -p "Source directory: " src && read -p "Password: " password ; \
	mkdir -p decrypt/$${src} && echo "\n" ; \
	for i in `ls encrypt/$${src}/*.cast5` ; do \
		echo "Decrypting $${i}" ; \
		openssl cast5-cbc -d -in $${i} -out decrypt/$${src}/`basename $${i%.*}` -pass pass:$${password}; \
		chmod 600 decrypt/$${src}/`basename $${i%.*}` ; \
	done ; \
	echo "Decrypted files are in decrypt/$${src}"

# Encrypt files in the decrypt/ directory
encrypt: encrypt-pwd-prompt
	@echo "Encrypt the files in a directory using a password you specify.  A directory will be created under /encrypt."
	@read -p "Source directory name: " src && read -p "Password: " password && echo "\n"; \
	mkdir -p encrypt/`basename $${src}` ; \
	echo "Encrypting $${src} ==> encrypt/`basename $${src}`" ; \
	for i in `ls $${src}` ; do \
		echo "Encrypting $${src}/$${i}" ; \
		openssl cast5-cbc -e -in $${src}/$${i} -out encrypt/`basename $${src}`/$${i}.cast5 -pass pass:$${password}; \
	done ; \
	echo "Encrypted files are in encrypt/`basename $${src}`"
