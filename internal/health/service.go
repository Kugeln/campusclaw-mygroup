package health

import "context"

type Database interface{ PingContext(context.Context) error }
type Service struct {
	DB    Database
	Probe func() error
}

func (s Service) Check(ctx context.Context) error {
	if err := s.DB.PingContext(ctx); err != nil {
		return err
	}
	return s.Probe()
}
