package printer

import (
    "bytes"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestListPrinter_SingleRow(t *testing.T) {
    buf := &bytes.Buffer{}
    lp, err := NewListPrinter(buf)
    require.NoError(t, err)

    lp.AddColumns("ID", "Name")
    lp.AddField("123")
    lp.AddField("Repo1")
    lp.EndRow()

    require.NoError(t, lp.Render())
    out := buf.String()
    require.Equal(t, "ID: 123\nName: Repo1\n", out)
}

func TestListPrinter_MultipleRows(t *testing.T) {
    buf := &bytes.Buffer{}
    lp, _ := NewListPrinter(buf)

    lp.AddColumns("ID", "Name")
    // Row 1
    lp.AddField("123")
    lp.AddField("Repo1")
    lp.EndRow()
    // Row 2
    lp.AddField("456")
    lp.AddField("Repo2")
    lp.EndRow()

    require.NoError(t, lp.Render())
    lines := strings.Split(buf.String(), "\n")
    // Expect blank line between objects
    require.Contains(t, buf.String(), "\n\n")
    require.Equal(t, "ID: 123", lines[0])
    require.Equal(t, "Name: Repo1", lines[1])
    require.Equal(t, "ID: 456", lines[3])
    require.Equal(t, "Name: Repo2", lines[4])
}

func TestListPrinter_MissingColumnName(t *testing.T) {
    buf := &bytes.Buffer{}
    lp, _ := NewListPrinter(buf)

    lp.AddColumns("ID")
    lp.AddField("123")
    lp.AddField("ExtraField") // no column name for this index
    lp.EndRow()

    require.NoError(t, lp.Render())
    out := buf.String()
    require.Contains(t, out, "col1: ExtraField")
}

func TestListPrinter_AddTimeField(t *testing.T) {
    buf := &bytes.Buffer{}
    lp, _ := NewListPrinter(buf)
    lp.AddColumns("Created")
    now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
    past := now.Add(-time.Hour)

    lp.AddTimeField(now, past, nil)
    lp.EndRow()

    require.NoError(t, lp.Render())
    out := buf.String()
    require.Contains(t, out, "Created:")
}

// Negative/edge case tests
func TestListPrinter_NoColumns(t *testing.T) {
    buf := &bytes.Buffer{}
    lp, _ := NewListPrinter(buf)

    lp.AddField("ValueWithoutHeader")
    lp.EndRow()

    // Even without columns, should render with col index fallback
    require.NoError(t, lp.Render())
    out := buf.String()
    require.Contains(t, out, "col0: ValueWithoutHeader")
}

func TestListPrinter_RenderEmpty(t *testing.T) {
    buf := &bytes.Buffer{}
    lp, _ := NewListPrinter(buf)

    // No rows added; Render should produce no error and empty output
    require.NoError(t, lp.Render())
    require.Equal(t, "", buf.String())
}
