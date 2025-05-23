// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"net/http"
	"path"

	"github.com/unknwon/com"
	log "unknwon.dev/clog/v2"

	"gogs.io/gogs/internal/context"
	"gogs.io/gogs/internal/database"
	"gogs.io/gogs/internal/form"
)

const (
	tmplOrgTeams            = "org/team/teams"
	tmplOrgTeamNew          = "org/team/new"
	tmplOrgTeamMembers      = "org/team/members"
	tmplOrgTeamRepositories = "org/team/repositories"
)

func Teams(c *context.Context) {
	org := c.Org.Organization
	c.Data["Title"] = org.FullName
	c.Data["PageIsOrgTeams"] = true

	for _, t := range org.Teams {
		if err := t.GetMembers(); err != nil {
			c.Error(err, "get members")
			return
		}
	}
	c.Data["Teams"] = org.Teams

	c.Success(tmplOrgTeams)
}

func TeamsAction(c *context.Context) {
	uid := com.StrTo(c.Query("uid")).MustInt64()
	if uid == 0 {
		c.Redirect(c.Org.OrgLink + "/teams")
		return
	}

	page := c.Query("page")
	var err error
	switch c.Params(":action") {
	case "join":
		if !c.Org.IsOwner {
			c.NotFound()
			return
		}
		err = c.Org.Team.AddMember(c.User.ID)
	case "leave":
		err = c.Org.Team.RemoveMember(c.User.ID)
	case "remove":
		if !c.Org.IsOwner {
			c.NotFound()
			return
		}
		err = c.Org.Team.RemoveMember(uid)
		page = "team"
	case "add":
		if !c.Org.IsOwner {
			c.NotFound()
			return
		}
		uname := c.Query("uname")
		var u *database.User
		u, err = database.Handle.Users().GetByUsername(c.Req.Context(), uname)
		if err != nil {
			if database.IsErrUserNotExist(err) {
				c.Flash.Error(c.Tr("form.user_not_exist"))
				c.Redirect(c.Org.OrgLink + "/teams/" + c.Org.Team.LowerName)
			} else {
				c.Error(err, "get user by name")
			}
			return
		}

		err = c.Org.Team.AddMember(u.ID)
		page = "team"
	}

	if err != nil {
		if database.IsErrLastOrgOwner(err) {
			c.Flash.Error(c.Tr("form.last_org_owner"))
		} else {
			log.Error("Action(%s): %v", c.Params(":action"), err)
			c.JSONSuccess(map[string]any{
				"ok":  false,
				"err": err.Error(),
			})
			return
		}
	}

	switch page {
	case "team":
		c.Redirect(c.Org.OrgLink + "/teams/" + c.Org.Team.LowerName)
	default:
		c.Redirect(c.Org.OrgLink + "/teams")
	}
}

func TeamsRepoAction(c *context.Context) {
	if !c.Org.IsOwner {
		c.NotFound()
		return
	}

	var err error
	switch c.Params(":action") {
	case "add":
		repoName := path.Base(c.Query("repo_name"))
		var repo *database.Repository
		repo, err = database.GetRepositoryByName(c.Org.Organization.ID, repoName)
		if err != nil {
			if database.IsErrRepoNotExist(err) {
				c.Flash.Error(c.Tr("org.teams.add_nonexistent_repo"))
				c.Redirect(c.Org.OrgLink + "/teams/" + c.Org.Team.LowerName + "/repositories")
				return
			}

			c.Error(err, "get repository by name")
			return
		}
		err = c.Org.Team.AddRepository(repo)
	case "remove":
		err = c.Org.Team.RemoveRepository(com.StrTo(c.Query("repoid")).MustInt64())
	}

	if err != nil {
		c.Errorf(err, "action %q", c.Params(":action"))
		return
	}
	c.Redirect(c.Org.OrgLink + "/teams/" + c.Org.Team.LowerName + "/repositories")
}

func NewTeam(c *context.Context) {
	c.Data["Title"] = c.Org.Organization.FullName
	c.Data["PageIsOrgTeams"] = true
	c.Data["PageIsOrgTeamsNew"] = true
	c.Data["Team"] = &database.Team{}
	c.Success(tmplOrgTeamNew)
}

func NewTeamPost(c *context.Context, f form.CreateTeam) {
	c.Data["Title"] = c.Org.Organization.FullName
	c.Data["PageIsOrgTeams"] = true
	c.Data["PageIsOrgTeamsNew"] = true

	t := &database.Team{
		OrgID:       c.Org.Organization.ID,
		Name:        f.TeamName,
		Description: f.Description,
		Authorize:   database.ParseAccessMode(f.Permission),
	}
	c.Data["Team"] = t

	if c.HasError() {
		c.Success(tmplOrgTeamNew)
		return
	}

	if err := database.NewTeam(t); err != nil {
		c.Data["Err_TeamName"] = true
		switch {
		case database.IsErrTeamAlreadyExist(err):
			c.RenderWithErr(c.Tr("form.team_name_been_taken"), tmplOrgTeamNew, &f)
		case database.IsErrNameNotAllowed(err):
			c.RenderWithErr(c.Tr("org.form.team_name_not_allowed", err.(database.ErrNameNotAllowed).Value()), tmplOrgTeamNew, &f)
		default:
			c.Error(err, "new team")
		}
		return
	}
	log.Trace("Team created: %s/%s", c.Org.Organization.Name, t.Name)
	c.Redirect(c.Org.OrgLink + "/teams/" + t.LowerName)
}

func TeamMembers(c *context.Context) {
	c.Data["Title"] = c.Org.Team.Name
	c.Data["PageIsOrgTeams"] = true
	if err := c.Org.Team.GetMembers(); err != nil {
		c.Error(err, "get members")
		return
	}
	c.Success(tmplOrgTeamMembers)
}

func TeamRepositories(c *context.Context) {
	c.Data["Title"] = c.Org.Team.Name
	c.Data["PageIsOrgTeams"] = true
	if err := c.Org.Team.GetRepositories(); err != nil {
		c.Error(err, "get repositories")
		return
	}
	c.Success(tmplOrgTeamRepositories)
}

func EditTeam(c *context.Context) {
	c.Data["Title"] = c.Org.Organization.FullName
	c.Data["PageIsOrgTeams"] = true
	c.Data["team_name"] = c.Org.Team.Name
	c.Data["desc"] = c.Org.Team.Description
	c.Success(tmplOrgTeamNew)
}

func EditTeamPost(c *context.Context, f form.CreateTeam) {
	t := c.Org.Team
	c.Data["Title"] = c.Org.Organization.FullName
	c.Data["PageIsOrgTeams"] = true
	c.Data["Team"] = t

	if c.HasError() {
		c.Success(tmplOrgTeamNew)
		return
	}

	isAuthChanged := false
	if !t.IsOwnerTeam() {
		// Validate permission level.
		var auth database.AccessMode
		switch f.Permission {
		case "read":
			auth = database.AccessModeRead
		case "write":
			auth = database.AccessModeWrite
		case "admin":
			auth = database.AccessModeAdmin
		default:
			c.Status(http.StatusUnauthorized)
			return
		}

		t.Name = f.TeamName
		if t.Authorize != auth {
			isAuthChanged = true
			t.Authorize = auth
		}
	}
	t.Description = f.Description
	if err := database.UpdateTeam(t, isAuthChanged); err != nil {
		c.Data["Err_TeamName"] = true
		switch {
		case database.IsErrTeamAlreadyExist(err):
			c.RenderWithErr(c.Tr("form.team_name_been_taken"), tmplOrgTeamNew, &f)
		default:
			c.Error(err, "update team")
		}
		return
	}
	c.Redirect(c.Org.OrgLink + "/teams/" + t.LowerName)
}

func DeleteTeam(c *context.Context) {
	if err := database.DeleteTeam(c.Org.Team); err != nil {
		c.Flash.Error("DeleteTeam: " + err.Error())
	} else {
		c.Flash.Success(c.Tr("org.teams.delete_team_success"))
	}

	c.JSONSuccess(map[string]any{
		"redirect": c.Org.OrgLink + "/teams",
	})
}
