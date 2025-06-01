circleci-rebuild:
	circleci config process .circleci/config.yml > process.yml

circleci-test:
	circleci local execute build

startdb:
	docker compose up -d

stopdb:
	docker compose down

deletedbfiles:
	rm -rf db/axonserver
	rm -rf db/mongo

cleardb: stopdb
cleardb: deletedbfiles
