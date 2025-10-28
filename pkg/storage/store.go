package storage

import "time"

type Snapshot struct {
	Workload        string
	Metric          string
	GeneratedAt     time.Time
	StepSeconds     int
	HorizonSeconds  int
	Values          []float64
	DesiredReplicas []int
}

type Store interface {
	Put(Snapshot) error
	GetLatest(workload string) (Snapshot, bool, error)
}
