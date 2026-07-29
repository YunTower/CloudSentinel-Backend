package controllers

import (
	"goravel/app/repositories"
)

// publicPagePolicy is the internal module for public-page configuration. Its
// small interface resolves a path to the validated page model and its incident
// scope, hiding legacy normalization and block compatibility from callers.
type publicPagePolicy struct{}

type publicPageResolution struct {
	Config         PublicPagesConfigV1
	Page           *PublicPageV1
	IncidentFilter *publicIncidentFilter
}

func getPublicPagePolicy() publicPagePolicy {
	return publicPagePolicy{}
}

func (p publicPagePolicy) Load() PublicPagesConfigV1 {
	repo := repositories.GetSystemSettingRepository()
	cfg := defaultPublicPagesConfigV1()
	_ = repo.GetJSONWithDefault(publicPagesSettingKeyV1, &cfg, defaultPublicPagesConfigV1())
	normalizePublicPagesConfigV1(&cfg)
	ensurePublicIncidentsSeparated(&cfg)
	return cfg
}

func (p publicPagePolicy) Resolve(path string) (*publicPageResolution, error) {
	cfg := p.Load()
	page, err := resolveBoundPageByPath(cfg, path)
	if err != nil {
		return nil, err
	}
	return &publicPageResolution{
		Config:         cfg,
		Page:           page,
		IncidentFilter: buildPublicIncidentFilter(page),
	}, nil
}

func (p publicPagePolicy) NormalizeValidate(cfg *PublicPagesConfigV1) error {
	normalizePublicPagesConfigV1(cfg)
	return validatePublicPagesConfigV1(cfg)
}

func (p publicPagePolicy) Save(cfg PublicPagesConfigV1) error {
	return repositories.GetSystemSettingRepository().SetJSON(publicPagesSettingKeyV1, cfg)
}
