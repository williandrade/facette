package backend

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"facette/template"

	"github.com/facette/maputil"
	"github.com/jinzhu/gorm"
)

// Graph represents a library graph item instance.
type Graph struct {
	Item
	Groups     SeriesGroups `gorm:"type:text;not null" json:"groups,omitempty"`
	Link       *Graph       `json:"-"`
	LinkID     *string      `gorm:"column:link;type:varchar(36)" json:"link,omitempty"`
	Attributes maputil.Map  `gorm:"type:text" json:"attributes,omitempty"`
	Alias      *string      `gorm:"type:varchar(128);unique_index" json:"alias,omitempty"`
	Options    maputil.Map  `gorm:"type:text" json:"options,omitempty"`
	Template   bool         `gorm:"not null" json:"template"`

	resolved bool
	expanded bool
}

// NewGraph creates a new back-end graph item instance.
func (b *Backend) NewGraph() *Graph {
	return &Graph{Item: Item{backend: b}}
}

// BeforeSave handles the ORM 'BeforeSave' callback.
func (g *Graph) BeforeSave(scope *gorm.Scope) error {
	if err := g.Item.BeforeSave(scope); err != nil {
		return err
	} else if g.Alias != nil && !nameRegexp.MatchString(*g.Alias) {
		return ErrInvalidAlias
	}

	// Ensure optional fields are null if empty
	if g.LinkID != nil && *g.LinkID == "" {
		scope.SetColumn("LinkID", nil)
	}

	if g.Alias != nil && *g.Alias == "" {
		scope.SetColumn("Alias", nil)
	}

	return nil
}

// Expand expands the graph item instance using its linked instance.
func (g *Graph) Expand(attrs maputil.Map) error {
	var err error

	if g.expanded {
		return nil
	}

	if len(attrs) > 0 {
		g.Attributes.Merge(attrs, true)
	}

	if g.backend != nil && g.LinkID != nil && *g.LinkID != "" {
		if err := g.Resolve(); err != nil {
			return err
		}

		// Expand template and applies current graph's attributes
		tmpl := g.Link
		tmpl.ID = g.ID
		tmpl.Attributes.Merge(g.Attributes, true)
		tmpl.Options.Merge(g.Options, true)
		tmpl.Template = false

		*g = *tmpl
	}

	if title, ok := g.Options["title"].(string); ok {
		if g.Options["title"], err = template.Expand(title, g.Attributes); err != nil {
			return err
		}
	}

	for _, group := range g.Groups {
		for _, series := range group.Series {
			if series.Name, err = template.Expand(series.Name, g.Attributes); err != nil {
				return err
			} else if series.Origin, err = template.Expand(series.Origin, g.Attributes); err != nil {
				return err
			} else if series.Source, err = template.Expand(series.Source, g.Attributes); err != nil {
				return err
			} else if series.Metric, err = template.Expand(series.Metric, g.Attributes); err != nil {
				return err
			}
		}
	}

	g.expanded = true

	return nil
}

// Resolve resolves the graph item linked instance.
func (g *Graph) Resolve() error {
	if g.resolved {
		return nil
	} else if g.backend == nil {
		return ErrUnresolvableItem
	}

	if g.LinkID != nil && *g.LinkID != "" {
		g.Link = g.backend.NewGraph()
		if err := g.backend.Storage().Get("id", *g.LinkID, g.Link); err != nil {
			return err
		}
	}

	g.resolved = true

	return nil
}

// SeriesGroups represents a list of library graph series groups.
type SeriesGroups []*SeriesGroup

// Value marshals the series groups for compatibility with SQL drivers.
func (l SeriesGroups) Value() (driver.Value, error) {
	data, err := json.Marshal(l)
	return data, err
}

// Scan unmarshals the series groups retrieved from SQL drivers.
func (l *SeriesGroups) Scan(v interface{}) error {
	return json.Unmarshal(v.([]byte), l)
}

// SeriesGroup represents a library graph series group entry instance.
type SeriesGroup struct {
	Name        string      `json:"name"`
	Operator    int         `json:"operator"`
	Consolidate int         `json:"consolidate"`
	Series      []*Series   `json:"series"`
	Options     maputil.Map `json:"options,omitempty"`
}

// Series represents a library graph series entry instance.
type Series struct {
	Name    string      `json:"name"`
	Origin  string      `json:"origin"`
	Source  string      `json:"source"`
	Metric  string      `json:"metric"`
	Options maputil.Map `json:"options,omitempty"`
}

// IsValid checks whether or not the series instance is valid.
func (s Series) IsValid() bool {
	return s.Origin != "" && s.Source != "" && s.Metric != ""
}

// String returns a string representation of the series instance.
func (s Series) String() string {
	return fmt.Sprintf("{Name: %q, Origin: %q, Source: %q, Metric: %q}", s.Name, s.Origin, s.Source, s.Metric)
}
