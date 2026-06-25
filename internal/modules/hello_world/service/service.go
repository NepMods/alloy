package service

import (
	"alloy/internal/platform/messaging"
	"alloy/models/contract"
	"context"
	"math/rand"
)

type Service struct {
	rt contract.Runtime
}

func New(rt contract.Runtime) *Service { return &Service{rt: rt} }

func (s *Service) LogFromHelloWorldAndGetARandomNumberBack(message string) int {
	s.rt.Bus().Publish(context.Background(), messaging.Message{
		Topic:   "log.info",
		Payload: "Hello worled from logger",
	})
	return rand.Int()
}
