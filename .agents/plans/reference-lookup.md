# Canonical Reference Lookup

## Goal

Give dex one lookup path that can turn ambiguous text, names, IDs, and URLs into canonical entity references across installed datasource plugins.

Examples:

- `timo` can resolve to `slack.user:U1234` and `gitlab.user:timo`.
- `https://gitlab.example.com/group/project/-/merge_requests/12` can resolve to `gitlab.merge_request:group/project!12`, plus related project, namespace, and author links when known.
- `#engineering` can resolve to `slack.channel:C1234`.

## Terms

- **Record**: indexed datasource record owned by one plugin instance.
- **Reference**: stable `{entity, id}` pair, with source `{plugin, instance}`.
- **Link**: named URL or related reference attached to a record.
- **Lookup**: operation over input text/terms that returns ranked canonical references.
- **Resolver**: plugin-side or host-side logic that extracts candidate references from text.

## Data Contract

Lookup results should keep the same top-level shape as host index lookup:

```json
{
  "source": "host_index",
  "text": "look at https://gitlab.example.com/group/project/-/merge_requests/12",
  "terms": ["timo"],
  "count": 1,
  "matches": [
    {
      "source": {"plugin": "gitlab", "instance": "default", "index": "gitlab.merge_requests"},
      "entity": "gitlab.merge_request",
      "id": "group/project!12",
      "score": 1000,
      "matched_fields": ["links.self"],
      "record": {
        "entity": "gitlab.merge_request",
        "id": "group/project!12",
        "title": "Ship change",
        "links": {
          "self": "https://gitlab.example.com/group/project/-/merge_requests/12",
          "project_entity": "gitlab.project:group/project"
        }
      }
    }
  ]
}
```

## Responsibilities

Host index lookup should:

- Search only installed and connected plugins that expose lookup/search/get capabilities.
- Match indexed canonical IDs, titles, URLs, links, and provider-specific record fields.
- Return ranked, source-qualified matches with stable top-level `entity` and `id`.
- Avoid duplicating `entity` and `id` inside raw record payloads.

Plugin lookup should:

- Be optional.
- Handle live or provider-specific parsing that cannot be done generically from indexed records.
- Return the same standardized datasource lookup result shape.
- Prefer `pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[T]]` for all plugin and host lookup responses.

## Resolver Strategy

1. **Exact URL/link match**: highest score. Match `links.self`, `url`, `web_url`, Slack synthetic URLs, and canonical entity links.
2. **Canonical entity ref**: parse `entity:id` strings exactly.
3. **Provider syntax**: parse known syntax like `project!iid`, `project#iid`, Slack channel IDs, user IDs, and channel names.
4. **Term search**: score name/title/username/display fields.
5. **Related references**: include relationship links already stored on records, but do not inflate them into separate matches unless explicitly requested later.

## Current Implementation

- `pluginbinding` owns the shared lookup source, match, and result types.
- `pluginbinding` also owns lookup candidate scoring, dedupe, sorting, limiting, and result assembly.
- Host index lookup aliases the shared match type and returns the same shape as plugin lookup.
- GitLab and Slack expose live datasource lookup handlers in addition to host-owned index lookup; provider code only fetches and normalizes provider records before handing candidates to `pluginbinding`.
- GitLab and Slack normalized records include canonical `self` links and relationship links where known.
- CLI search and lookup tests assert source-qualified records, canonical links, and stable top-level `{entity, id}` matches.

## Next Implementation Steps

1. Decide whether follow-up expansion should return related records inline or only as named links.
2. Add more provider-specific live lookup fetches when clients expose efficient exact lookups, especially GitLab issues/groups and Slack permalink/message resolution.
3. Add user-facing text/compact output once the JSON contract settles.
