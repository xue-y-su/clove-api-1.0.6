package model_catalog_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStoredDefaultsAndMigratesLegacyVisibility(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		got, err := ParseStored(raw)
		require.NoError(t, err)
		assert.Equal(t, DefaultVisibility(), got)
	}

	got, err := ParseStored(`{"version":1,"mode":"all","groups":[]}`)
	require.NoError(t, err)
	assert.Equal(t, EnabledVisibility(), got)

	got, err = ParseStored(`{"version":1,"mode":"selected","groups":["test","default","test"]}`)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, DefaultModeHidden, got.DefaultMode)
	assert.Equal(t, map[string]GroupRule{
		"default": {Mode: ModeAll, UserGroups: []string{}},
		"test":    {Mode: ModeAll, UserGroups: []string{}},
	}, got.Rules)

	got, err = ParseStored(`{"version":2,"default_mode":"all","rules":{"test":{"mode":"selected","user_groups":["Admin"]}}}`)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, CurrentVersion, got.Version)
	assert.Equal(t, []string{"Admin"}, got.Rules["test"].UserGroups)

	_, err = ParseStored(`{"version":4,"enabled":true,"default_mode":"all","rules":{}}`)
	require.ErrorContains(t, err, "version")
	_, err = ParseStored(`{"version":3,"enabled":true,"default_mode":"broken","rules":{}}`)
	require.ErrorContains(t, err, "default")
	_, err = ParseStored(`{"version":3,"enabled":true,"default_mode":"all","rules":{},"extra":true}`)
	require.ErrorContains(t, err, "unknown field")
	_, err = ParseStored(`{"version":3,"default_mode":"all","rules":{}}`)
	require.ErrorContains(t, err, "enabled")
	_, err = ParseStored(`{"version":3,"enabled":true,"default_mode":"all","rules":{}} {}`)
	require.ErrorContains(t, err, "exactly one")
}

func TestValidateUpdateNormalizesPerCatalogGroupRules(t *testing.T) {
	current := Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules: map[string]GroupRule{
			"deleted": {Mode: ModeSelected, UserGroups: []string{"removed-user"}},
		},
	}
	catalogUniverse := map[string]string{"default": "Default", "vip": "VIP", "test": "Test"}
	userGroupUniverse := map[string]string{"default": "Default", "vip": "VIP", "test": "Test", "Admin": "Admin"}
	got, err := ValidateUpdate(Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules: map[string]GroupRule{
			"test":    {Mode: ModeSelected, UserGroups: []string{"test", "Admin", "test"}},
			"deleted": {Mode: ModeSelected, UserGroups: []string{"removed-user"}},
		},
	}, current, catalogUniverse, userGroupUniverse)
	require.NoError(t, err)
	assert.Equal(t, []string{"Admin", "test"}, got.Rules["test"].UserGroups)
	assert.Equal(t, []string{"removed-user"}, got.Rules["deleted"].UserGroups)

	_, err = ValidateUpdate(Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules:       map[string]GroupRule{"invented": {Mode: ModeAll}},
	}, current, catalogUniverse, userGroupUniverse)
	require.ErrorContains(t, err, "invented")

	_, err = ValidateUpdate(Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules: map[string]GroupRule{
			"test": {Mode: ModeSelected, UserGroups: []string{"invented-user"}},
		},
	}, current, catalogUniverse, userGroupUniverse)
	require.ErrorContains(t, err, "invented-user")
}

func TestVisibleToUserUsesSeparateRulesForEveryCatalogGroup(t *testing.T) {
	visibility := Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules: map[string]GroupRule{
			"test":   {Mode: ModeSelected, UserGroups: []string{"Admin", "test"}},
			"secret": {Mode: ModeSelected, UserGroups: []string{}},
		},
	}
	assert.True(t, VisibleToUser(visibility, "test", "Admin"))
	assert.True(t, VisibleToUser(visibility, "test", "test"))
	assert.False(t, VisibleToUser(visibility, "test", "default"))
	assert.False(t, VisibleToUser(visibility, "secret", "Admin"))
	assert.True(t, VisibleToUser(visibility, "default", "default"))

	visibility.Enabled = false
	assert.True(t, VisibleToUser(visibility, "test", "Admin"))
	assert.True(t, VisibleToUser(visibility, "default", "default"))
	assert.Equal(t, ModeAll, EffectiveRule(visibility, "test").Mode)
	assert.Empty(t, EffectiveRule(visibility, "test").UserGroups)
	assert.Contains(t, visibility.Rules, "test")
}

func TestMarshalAndETagAreStableAfterNormalization(t *testing.T) {
	first, err := Marshal(Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules: map[string]GroupRule{
			"test": {Mode: ModeSelected, UserGroups: []string{"test", "Admin", "test"}},
		},
	})
	require.NoError(t, err)
	second, err := Marshal(Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules: map[string]GroupRule{
			"test": {Mode: ModeSelected, UserGroups: []string{"Admin", "test"}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"version":3,"enabled":true,"default_mode":"all","rules":{"test":{"mode":"selected","user_groups":["Admin","test"]}}}`, first)
	assert.Equal(t, first, second)
	assert.Equal(t, ETag(first), ETag("  "+second+"  "))
}
