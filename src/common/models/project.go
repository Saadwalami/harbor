// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/goharbor/harbor/src/pkg/quota/types"
	"github.com/goharbor/harbor/src/replication/model"
)

const (
	// ProjectTable is the table name for project
	ProjectTable = "project"
	// ProjectPublic means project is public
	ProjectPublic = "public"
	// ProjectPrivate means project is private
	ProjectPrivate = "private"
)

// Project holds the details of a project.
type Project struct {
	ProjectID    int64             `orm:"pk;auto;column(project_id)" json:"project_id"`
	OwnerID      int               `orm:"column(owner_id)" json:"owner_id"`
	Name         string            `orm:"column(name)" json:"name"`
	CreationTime time.Time         `orm:"column(creation_time);auto_now_add" json:"creation_time"`
	UpdateTime   time.Time         `orm:"column(update_time);auto_now" json:"update_time"`
	Deleted      bool              `orm:"column(deleted)" json:"deleted"`
	OwnerName    string            `orm:"-" json:"owner_name"`
	Role         int               `orm:"-" json:"current_user_role_id"`
	RoleList     []int             `orm:"-" json:"current_user_role_ids"`
	RepoCount    int64             `orm:"-" json:"repo_count"`
	ChartCount   uint64            `orm:"-" json:"chart_count"`
	Metadata     map[string]string `orm:"-" json:"metadata"`
	CVEAllowlist CVEAllowlist      `orm:"-" json:"cve_allowlist"`
	RegistryID   int64             `orm:"column(registry_id)" json:"registry_id"`
}

// GetMetadata ...
func (p *Project) GetMetadata(key string) (string, bool) {
	if len(p.Metadata) == 0 {
		return "", false
	}
	value, exist := p.Metadata[key]
	return value, exist
}

// SetMetadata ...
func (p *Project) SetMetadata(key, value string) {
	if p.Metadata == nil {
		p.Metadata = map[string]string{}
	}
	p.Metadata[key] = value
}

// IsPublic ...
func (p *Project) IsPublic() bool {
	public, exist := p.GetMetadata(ProMetaPublic)
	if !exist {
		return false
	}

	return isTrue(public)
}

// ContentTrustEnabled ...
func (p *Project) ContentTrustEnabled() bool {
	enabled, exist := p.GetMetadata(ProMetaEnableContentTrust)
	if !exist {
		return false
	}
	return isTrue(enabled)
}

// VulPrevented ...
func (p *Project) VulPrevented() bool {
	prevent, exist := p.GetMetadata(ProMetaPreventVul)
	if !exist {
		return false
	}
	return isTrue(prevent)
}

// ReuseSysCVEAllowlist ...
func (p *Project) ReuseSysCVEAllowlist() bool {
	r, ok := p.GetMetadata(ProMetaReuseSysCVEAllowlist)
	if !ok {
		return true
	}
	return isTrue(r)
}

// Severity ...
func (p *Project) Severity() string {
	severity, exist := p.GetMetadata(ProMetaSeverity)
	if !exist {
		return ""
	}
	return severity
}

// AutoScan ...
func (p *Project) AutoScan() bool {
	auto, exist := p.GetMetadata(ProMetaAutoScan)
	if !exist {
		return false
	}
	return isTrue(auto)
}

// FilterByPublic returns orm.QuerySeter with public filter
func (p *Project) FilterByPublic(ctx context.Context, qs orm.QuerySeter, key string, value interface{}) orm.QuerySeter {
	subQuery := `SELECT project_id FROM project_metadata WHERE name = 'public' AND value = '%s'`
	if isTrue(value) {
		subQuery = fmt.Sprintf(subQuery, "true")
	} else {
		subQuery = fmt.Sprintf(subQuery, "false")
	}
	return qs.FilterRaw("project_id", fmt.Sprintf("IN (%s)", subQuery))
}

// FilterByOwner returns orm.QuerySeter with owner filter
func (p *Project) FilterByOwner(ctx context.Context, qs orm.QuerySeter, key string, value string) orm.QuerySeter {
	return qs.FilterRaw("owner_id", fmt.Sprintf("IN (SELECT user_id FROM harbor_user WHERE username = '%s')", value))
}

// FilterByMember returns orm.QuerySeter with member filter
func (p *Project) FilterByMember(ctx context.Context, qs orm.QuerySeter, key string, value interface{}) orm.QuerySeter {
	query, ok := value.(*MemberQuery)
	if !ok {
		return qs
	}
	subQuery := `SELECT project_id FROM project_member pm, harbor_user u WHERE pm.entity_id = u.user_id AND pm.entity_type = 'u' AND u.username = '%s'`
	subQuery = fmt.Sprintf(subQuery, escape(query.Name))
	if query.Role > 0 {
		subQuery = fmt.Sprintf("%s AND pm.role = %d", subQuery, query.Role)
	}

	if query.WithPublic {
		subQuery = fmt.Sprintf("(%s) UNION (SELECT project_id FROM project_metadata WHERE name = 'public' AND value = 'true')", subQuery)
	}

	if len(query.GroupIDs) > 0 {
		var elems []string
		for _, groupID := range query.GroupIDs {
			elems = append(elems, strconv.Itoa(groupID))
		}

		tpl := "(%s) UNION (SELECT project_id FROM project_member pm, user_group ug WHERE pm.entity_id = ug.id AND pm.entity_type = 'g' AND ug.id IN (%s))"
		subQuery = fmt.Sprintf(tpl, subQuery, strings.TrimSpace(strings.Join(elems, ", ")))
	}

	return qs.FilterRaw("project_id", fmt.Sprintf("IN (%s)", subQuery))
}

func isTrue(i interface{}) bool {
	switch value := i.(type) {
	case bool:
		return value
	case string:
		v := strings.ToLower(value)
		return v == "true" || v == "1"
	default:
		return false
	}
}

func escape(str string) string {
	// TODO: remove this function when resolve the cycle import between lib/orm, common/dao and common/models.
	// We need to move the function to registry the database from common/dao to lib/database,
	// then lib/orm no need to depend the PrepareTestForPostgresSQL method in the common/dao
	str = strings.Replace(str, `\`, `\\`, -1)
	str = strings.Replace(str, `%`, `\%`, -1)
	str = strings.Replace(str, `_`, `\_`, -1)
	return str
}

// ProjectQueryParam can be used to set query parameters when listing projects.
// The query condition will be set in the query if its corresponding field
// is not nil. Leave it empty if you don't want to apply this condition.
//
// e.g.
// List all projects: query := nil
// List all public projects: query := &QueryParam{Public: true}
// List projects the owner of which is user1: query := &QueryParam{Owner:"user1"}
// List all public projects the owner of which is user1: query := &QueryParam{Owner:"user1",Public:true}
// List projects which user1 is member of: query := &QueryParam{Member:&Member{Name:"user1"}}
// List projects which user1 is the project admin : query := &QueryParam{Member:&Member{Name:"user1",Role:1}}
type ProjectQueryParam struct {
	Name       string // the name of project
	Owner      string // the username of project owner
	Public     *bool  // the project is public or not, can be ture, false and nil
	RegistryID int64
	Member     *MemberQuery // the member of project
	Pagination *Pagination  // pagination information
	ProjectIDs []int64      // project ID list
}

// MemberQuery filter by member's username and role
type MemberQuery struct {
	Name     string // the username of member
	Role     int    // the role of the member has to the project
	GroupIDs []int  // the group ID of current user belongs to

	WithPublic bool // include the public projects for the member
}

// Pagination ...
type Pagination struct {
	Page int64
	Size int64
}

// Sorting sort by given field, ascending or descending
type Sorting struct {
	Sort string // in format [+-]?<FIELD_NAME>, e.g. '+creation_time', '-creation_time'
}

// BaseProjectCollection contains the query conditions which can be used
// to get a project collection. The collection can be used as the base to
// do other filter
type BaseProjectCollection struct {
	Public bool
	Member string
}

// ProjectRequest holds informations that need for creating project API
type ProjectRequest struct {
	Name         string            `json:"project_name"`
	Public       *int              `json:"public"` // deprecated, reserved for project creation in replication
	Metadata     map[string]string `json:"metadata"`
	CVEAllowlist CVEAllowlist      `json:"cve_allowlist"`

	StorageLimit *int64 `json:"storage_limit,omitempty"`
	RegistryID   int64  `json:"registry_id"`
}

// ProjectQueryResult ...
type ProjectQueryResult struct {
	Total    int64
	Projects []*Project
}

// TableName is required by beego orm to map Project to table project
func (p *Project) TableName() string {
	return ProjectTable
}

// QuotaSummary ...
type QuotaSummary struct {
	Hard types.ResourceList `json:"hard"`
	Used types.ResourceList `json:"used"`
}

// ProjectSummary ...
type ProjectSummary struct {
	RepoCount  int64  `json:"repo_count"`
	ChartCount uint64 `json:"chart_count"`

	ProjectAdminCount int64 `json:"project_admin_count"`
	MaintainerCount   int64 `json:"maintainer_count"`
	DeveloperCount    int64 `json:"developer_count"`
	GuestCount        int64 `json:"guest_count"`
	LimitedGuestCount int64 `json:"limited_guest_count"`

	Quota    *QuotaSummary   `json:"quota,omitempty"`
	Registry *model.Registry `json:"registry"`
}
