package source

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func getActiveAlerts(address string) ([]v1.Alert, error) {
	client, err := api.NewClient(api.Config{
		Address: address,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating client, %w", err)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	alerts, err := v1api.Alerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying prometheus: %w", err)
	}

	return alerts.Alerts, nil
}
