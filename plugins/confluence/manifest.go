package confluence

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	PluginName        = "confluence"
	PluginVersion     = "0.9.0"
	PluginDescription = "Confluence Cloud page/user operations, attachments, datasources, indexes, and reverse lookups."

	AuthMethodAtlassianCloud = "atlassian_cloud_basic"
	AuthPurposeAPIToken      = "api_token"
	AuthPurposeCloudID       = "cloud_id"

	EnvAtlassianAPIToken  = "ATLASSIAN_API_TOKEN"
	EnvAtlassianCloudID   = "ATLASSIAN_CLOUD_ID"
	EnvConfluenceAPIToken = "CONFLUENCE_API_TOKEN"
	EnvConfluenceCloudID  = "CONFLUENCE_CLOUD_ID"

	OperationAuthTest         = "confluence.auth.test"
	OperationIndexBuild       = "confluence.index.build"
	OperationAttachmentAdd    = "confluence.page.attachment.add"
	OperationAttachmentList   = "confluence.page.attachment.list"
	OperationAttachmentGet    = "confluence.attachment.get"
	OperationAttachmentDelete = "confluence.attachment.delete"
	OperationPageCreate       = "confluence.page.create"
	OperationPageDelete       = "confluence.page.delete"
	OperationPageSearch       = "confluence.page.search"
	OperationPageShow         = "confluence.page.show"
	OperationUserSearch       = "confluence.user.search"

	DatasourcePages = "confluence.pages"
	DatasourceUsers = "confluence.users"

	EntityPage = "confluence.page"
	EntityUser = "confluence.user"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"conf", PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodAtlassianCloud,
			Kind:        "bearer_token",
			Description: "Atlassian Cloud API token and cloud ID resolved by dex secret broker.",
			Env:         []string{EnvConfluenceAPIToken, EnvAtlassianAPIToken, EnvConfluenceCloudID, EnvAtlassianCloudID},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeAPIToken, "Atlassian API token", true, true, EnvConfluenceAPIToken, EnvAtlassianAPIToken),
				pluginbinding.AuthField(AuthPurposeCloudID, "Atlassian Cloud ID", true, false, EnvConfluenceCloudID, EnvAtlassianCloudID),
			},
		}},
		Operations: operationSpecs(),
		IndexedDatasources: []pluginbinding.IndexedDatasourceSpec{
			pluginbinding.IndexedDatasourceWithOptions(DatasourcePages, EntityPage, "Search Confluence pages.", "Confluence page reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[PageRecord](),
				pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "page_id", TitleField: "title"}),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
				pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceUsers, EntityUser, "Search Confluence users.", "Confluence user reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[UserRecord](),
				pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "account_id", TitleField: "display_name"}),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
				pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
			),
		},
	}
}

func operationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		authTestSpec(),
		indexBuildSpec(),
		attachmentAddSpec(),
		attachmentListSpec(),
		attachmentGetSpec(),
		attachmentDeleteSpec(),
		pageCreateSpec(),
		pageDeleteSpec(),
		pageSearchSpec(),
		pageShowSpec(),
		userSearchSpec(),
	}
}

func authTestSpec() core.OperationSpec {
	return confluenceReadOperation[NoInput, AuthTestResult](OperationAuthTest, "Test Confluence authentication by fetching the current user.")
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](OperationIndexBuild, "Build Confluence page and user index records.", confluenceReadOptions(core.OperationConditional)...)
}

func attachmentAddSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentAddInput, AttachmentUploadResult](OperationAttachmentAdd, "Upload an attachment to a Confluence page.", confluenceFilesystemWriteOptions(core.OperationNonIdempotent)...)
}

func attachmentListSpec() core.OperationSpec {
	return confluenceReadOperation[AttachmentListInput, AttachmentListResult](OperationAttachmentList, "List Confluence page attachments.")
}

func attachmentGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentGetInput, AttachmentGetResult](OperationAttachmentGet, "Download or return a Confluence attachment.", confluenceFilesystemReadOptions(core.OperationIdempotent)...)
}

func attachmentDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentDeleteInput, AttachmentDeleteResult](OperationAttachmentDelete, "Delete a Confluence attachment.", confluenceDeleteOptions(core.OperationNonIdempotent)...)
}

func pageCreateSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PageCreateInput, PageMutationResult](OperationPageCreate, "Create a Confluence page with storage-format content.", confluenceWriteOptions(core.OperationNonIdempotent)...)
}

func pageDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PageDeleteInput, PageMutationResult](OperationPageDelete, "Delete a Confluence page.", confluenceDeleteOptions(core.OperationNonIdempotent)...)
}

func pageSearchSpec() core.OperationSpec {
	return confluenceCompactReadOperation[PageSearchInput, PageSearchResult](OperationPageSearch, "Search Confluence pages.")
}

func pageShowSpec() core.OperationSpec {
	return confluenceReadOperation[PageShowInput, pluginbinding.ShowResult[Page]](OperationPageShow, "Show one Confluence page.")
}

func userSearchSpec() core.OperationSpec {
	return confluenceCompactReadOperation[UserSearchInput, UserSearchResult](OperationUserSearch, "Search Confluence users.")
}

func confluenceReadOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, confluenceReadOptions(core.OperationIdempotent)...)
}

func confluenceCompactReadOperation[I any, O any](name, description string) core.OperationSpec {
	options := append(confluenceReadOptions(core.OperationIdempotent), pluginbinding.Compact())
	return pluginbinding.TypedOperationSpec[I, O](name, description, options...)
}

func confluenceReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceFilesystemReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceFilesystemWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceDeleteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskDestructive),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluencePagesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[PageSearchInput, PageDatasourceResult](
		DatasourcePages,
		EntityPage,
		"Search Confluence pages.",
		[]string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.EntitySchemaFor[PageRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "page_id", TitleField: "title"}),
		pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
		pluginbinding.Completion("Confluence page fields.", "page_id", "title", "space_id", "space_key", "author_id"),
	)
}

func confluenceUsersDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[UserSearchInput, UserDatasourceResult](
		DatasourceUsers,
		EntityUser,
		"Search Confluence users.",
		[]string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.EntitySchemaFor[UserRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "account_id", TitleField: "display_name"}),
		pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
		pluginbinding.Completion("Confluence user fields.", "account_id", "display_name", "email"),
	)
}

func confluencePagesLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourcePages, EntityPage, "Lookup Confluence pages.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...))
}

func confluenceUsersLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourceUsers, EntityUser, "Lookup Confluence users.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...))
}

func atlassianAuthPurposes() []string {
	return []string{AuthPurposeAPIToken, AuthPurposeCloudID}
}
