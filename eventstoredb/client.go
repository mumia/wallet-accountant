package eventstoredb

import (
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"os"
)

const connectionStringName = "EVENTSTORE_CONNETION_STRING"

func NewClient() (*esdb.Client, error) {
	configuration, err := esdb.ParseConnectionString(os.Getenv(connectionStringName))
	if err != nil {
		return nil, err
	}

	client, err := esdb.NewClient(configuration)
	if err != nil {
		return nil, err
	}

	return client, nil
}
