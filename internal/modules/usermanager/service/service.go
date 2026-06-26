package service

import (
	"context"
	"time"

	"alloy/internal/platform/cache"
	"alloy/internal/modules/usermanager/store"
	"alloy/models/usermanager"
)

type Service struct {
	BaseService
	Store *store.Store
}

const cacheTTL = 5 * time.Minute

func (s *Service) ctx() context.Context {
	if s.Runtime == nil {
		return context.Background()
	}
	return s.Runtime.Context()
}

func (s *Service) Create(email, passwordHash, name string) (usermanager.User, error) {
	user, err := s.Store.Create(email, passwordHash, name)
	if err != nil {
		return user, err
	}
	s.Cache.Set(s.ctx(), "email:"+user.Email, user, cacheTTL)
	s.Cache.Set(s.ctx(), "id:"+itoa(user.ID), user, cacheTTL)
	return user, nil
}

func (s *Service) GetByEmail(email string) (usermanager.User, error) {
	user, err := cache.Get[usermanager.User](s.Cache, s.ctx(), "email:"+email)
	if err == nil {
		if user.PasswordHash != "" {
			return user, nil
		}
		s.Cache.Del(s.ctx(), "email:"+email)
	}
	user, err = s.Store.GetByEmail(email)
	if err != nil {
		return user, err
	}
	s.Cache.Set(s.ctx(), "email:"+user.Email, user, cacheTTL)
	s.Cache.Set(s.ctx(), "id:"+itoa(user.ID), user, cacheTTL)
	return user, nil
}

func (s *Service) GetByID(id int64) (usermanager.User, error) {
	user, err := cache.Get[usermanager.User](s.Cache, s.ctx(), "id:"+itoa(id))
	if err == nil {
		if user.PasswordHash != "" {
			return user, nil
		}
		s.Cache.Del(s.ctx(), "id:"+itoa(id))
	}
	user, err = s.Store.GetByID(id)
	if err != nil {
		return user, err
	}
	s.Cache.Set(s.ctx(), "email:"+user.Email, user, cacheTTL)
	s.Cache.Set(s.ctx(), "id:"+itoa(user.ID), user, cacheTTL)
	return user, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
