package gitbase

import (
	"io"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-mysql-server.v0/sql"
)

type remotesTable struct{}

// RemotesSchema is the schema for the remotes table.
var RemotesSchema = sql.Schema{
	{Name: "repository_id", Type: sql.Text, Nullable: false, Source: RemotesTableName},
	{Name: "remote_name", Type: sql.Text, Nullable: false, Source: RemotesTableName},
	{Name: "remote_push_url", Type: sql.Text, Nullable: false, Source: RemotesTableName},
	{Name: "remote_fetch_url", Type: sql.Text, Nullable: false, Source: RemotesTableName},
	{Name: "remote_push_refspec", Type: sql.Text, Nullable: false, Source: RemotesTableName},
	{Name: "remote_fetch_refspec", Type: sql.Text, Nullable: false, Source: RemotesTableName},
}

var _ sql.PushdownProjectionAndFiltersTable = (*remotesTable)(nil)

func newRemotesTable() sql.Table {
	return new(remotesTable)
}

var _ Table = (*remotesTable)(nil)

func (remotesTable) isGitbaseTable() {}

func (remotesTable) Resolved() bool {
	return true
}

func (remotesTable) Name() string {
	return RemotesTableName
}

func (remotesTable) Schema() sql.Schema {
	return RemotesSchema
}

func (r remotesTable) String() string {
	return printTable(RemotesTableName, RemotesSchema)
}

func (r *remotesTable) TransformUp(f sql.TransformNodeFunc) (sql.Node, error) {
	return f(r)
}

func (r *remotesTable) TransformExpressionsUp(f sql.TransformExprFunc) (sql.Node, error) {
	return r, nil
}

func (r remotesTable) RowIter(ctx *sql.Context) (sql.RowIter, error) {
	span, ctx := ctx.Span("gitbase.RemotesTable")
	iter := new(remotesIter)

	rowRepoIter, err := NewRowRepoIter(ctx, iter)
	if err != nil {
		span.Finish()
		return nil, err
	}

	return sql.NewSpanIter(span, rowRepoIter), nil
}

func (remotesTable) Children() []sql.Node {
	return nil
}

func (remotesTable) HandledFilters(filters []sql.Expression) []sql.Expression {
	return handledFilters(RemotesTableName, RemotesSchema, filters)
}

func (r *remotesTable) WithProjectAndFilters(
	ctx *sql.Context,
	_, filters []sql.Expression,
) (sql.RowIter, error) {
	span, ctx := ctx.Span("gitbase.RemotesTable")
	iter, err := rowIterWithSelectors(
		ctx, RemotesSchema, RemotesTableName, filters, nil,
		func(selectors) (RowRepoIter, error) {
			// it's not worth to manually filter with the selectors
			return new(remotesIter), nil
		},
	)

	if err != nil {
		span.Finish()
		return nil, err
	}

	return sql.NewSpanIter(span, iter), nil
}

type remotesIter struct {
	repositoryID string
	remotes      []*git.Remote
	remotePos    int
	urlPos       int
}

func (i *remotesIter) NewIterator(repo *Repository) (RowRepoIter, error) {

	remotes, err := repo.Repo.Remotes()
	if err != nil {
		return nil, err
	}

	return &remotesIter{
		repositoryID: repo.ID,
		remotes:      remotes,
		remotePos:    0,
		urlPos:       0}, nil
}

func (i *remotesIter) Next() (sql.Row, error) {
	if i.remotePos >= len(i.remotes) {
		return nil, io.EOF
	}

	remote := i.remotes[i.remotePos]
	config := remote.Config()

	if i.urlPos >= len(config.URLs) {
		i.remotePos++
		if i.remotePos >= len(i.remotes) {
			return nil, io.EOF
		}

		remote = i.remotes[i.remotePos]
		config = remote.Config()
		i.urlPos = 0
	}

	row := sql.NewRow(
		i.repositoryID,
		config.Name,
		config.URLs[i.urlPos],
		config.URLs[i.urlPos],
		config.Fetch[i.urlPos].String(),
		config.Fetch[i.urlPos].String(),
	)

	i.urlPos++

	return row, nil
}

func (i *remotesIter) Close() error {
	return nil
}
