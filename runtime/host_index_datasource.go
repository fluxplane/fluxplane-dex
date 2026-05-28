package runtime

import (
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type hostIndexDatasource struct {
	state    State
	plugin   string
	instance string
}

func (ds hostIndexDatasource) Response(command string, payload any) (protocol.Response, bool, error) {
	hasRecords, err := ds.state.HasIndexRecords(ds.plugin, ds.instance)
	if err != nil {
		return protocol.Response{}, false, err
	}
	if !hasRecords {
		return protocol.Response{}, false, nil
	}
	switch command {
	case protocol.CommandDatasourcesSearch:
		return ds.search(payload)
	case protocol.CommandDatasourcesLookup:
		return ds.lookup(payload)
	case protocol.CommandDatasourcesGet:
		return ds.get(payload)
	default:
		return protocol.Response{}, false, nil
	}
}

func (ds hostIndexDatasource) search(payload any) (protocol.Response, bool, error) {
	options := searchPayload(payload)
	records, err := ds.state.SearchIndexWithOptions(ds.plugin, ds.instance, options)
	if err != nil {
		return protocol.Response{}, false, err
	}
	return protocol.OK(pluginbinding.NewDatasourceSearchResult("host_index", options.Query, records)), true, nil
}

func (ds hostIndexDatasource) lookup(payload any) (protocol.Response, bool, error) {
	options := lookupPayload(payload)
	matches, err := ds.state.LookupIndexWithOptions(ds.plugin, ds.instance, options)
	if err != nil {
		return protocol.Response{}, false, err
	}
	return protocol.OK(pluginbinding.NewDatasourceLookupResult("host_index", options.Text, options.Terms, matches)), true, nil
}

func (ds hostIndexDatasource) get(payload any) (protocol.Response, bool, error) {
	id := getPayloadID(payload)
	if id == "" {
		return protocol.Fail("bad_payload", "datasource get requires id"), true, nil
	}
	record, ok, err := ds.state.GetIndexRecord(ds.plugin, ds.instance, id)
	if err != nil {
		return protocol.Response{}, false, err
	}
	if !ok {
		return protocol.Fail("not_found", "indexed record not found"), true, nil
	}
	return protocol.OK(pluginbinding.NewDatasourceGetResult("host_index", record)), true, nil
}

func isDatasourceCommand(command string) bool {
	switch command {
	case protocol.CommandDatasourcesSearch, protocol.CommandDatasourcesLookup, protocol.CommandDatasourcesGet:
		return true
	default:
		return false
	}
}
