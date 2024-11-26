circleci-rebuild:
	circleci config process .circleci/config.yml > process.yml

circleci-test:
	circleci local execute build
