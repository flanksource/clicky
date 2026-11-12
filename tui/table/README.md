# Shared terminal tables

The `table.Model` consumes Clicky's `api.TextTable` and caller-supplied stable row IDs. It has no Bubble Tea dependency and can be used from either major version. The host owns data fetching, native query filters, ordering and sample navigation.

```go
var view table.Model
if err := view.Set(api.NewTableFrom(rows), rowIDs); err != nil {
    return err
}
view.SetSize(80, 20) // e.g. from tea.WindowSizeMsg
view.Key("down")
output := view.View()
```

`Set` rejects missing or duplicate identities and keeps the selected identity through reordering. `SetSize(width, height)` records the viewport in terminal cells. `View()` prioritizes declared column order, hides default-hidden columns, respects column width limits and clips by terminal cells. Put identity, primary metric and status first. Additional fields belong in `RowDetail()` as well as the column picker of a web consumer.

Pass `enter` to open detail, `esc` to close it, and arrows or `j/k` to move or scroll. Detail scrolling stops at the last line that fits the viewport. A model in detail mode retains its inspected table when `Set` is called. The host should pin the corresponding sample and require an explicit resume action before applying a newer sample. `Selected`, `Detail` and `DetailOffset` are available for controlled host state.

```sh
go test ./tui/table
```

The focused tests cover stable selection, frozen detail, Unicode cell widths, narrow/wide terminals and scrolling through the final detail line.
