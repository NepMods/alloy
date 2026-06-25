package service

import "alloy/internal/modules/hello_world"

type Service struct{ hwlogger hello_world.HelloWorldLogger }

func New(hwlogger hello_world.HelloWorldLogger) *Service { return &Service{hwlogger: hwlogger} }

func (s *Service) Add(count int) int {
	return s.hwlogger.LogFromHelloWorldAndGetARandomNumberBack("Hello world from counter")
}
