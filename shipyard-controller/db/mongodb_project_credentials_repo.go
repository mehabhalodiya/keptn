package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	apimodels "github.com/keptn/go-utils/pkg/api/models"
	"github.com/keptn/keptn/shipyard-controller/models"
	"go.mongodb.org/mongo-driver/bson"
)

type MongoDBProjectCredentialsRepo interface {
	UpdateProject(project *models.ExpandedProjectOld) error
	GetOldCredentialsProjects() ([]*models.ExpandedProjectOld, error)
}

type mongoDBProjectCredentialsRepo struct {
	ProjectRepo *MongoDBProjectsRepo
}

func NewMongoDBProjectCredentialsRepo(dbConnection *MongoDBConnection) *mongoDBProjectCredentialsRepo {
	projectsRepo := NewMongoDBProjectsRepo(dbConnection)
	return &mongoDBProjectCredentialsRepo{
		ProjectRepo: projectsRepo,
	}
}

func (m *mongoDBProjectCredentialsRepo) GetOldCredentialsProjects() ([]*models.ExpandedProjectOld, error) {
	result := []*models.ExpandedProjectOld{}
	err := m.ProjectRepo.DBConnection.EnsureDBConnection()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	projectCollection := m.ProjectRepo.getProjectsCollection()
	cursor, err := projectCollection.Find(ctx, bson.M{})
	if err != nil {
		fmt.Println("Error retrieving projects from mongoDB: " + err.Error())
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		projectResult := &models.ExpandedProjectOld{}
		err := cursor.Decode(projectResult)
		if err != nil {
			fmt.Println("Could not cast to *models.Project")
		}
		result = append(result, projectResult)
	}

	return result, nil
}

func TransformGitCredentials(project *models.ExpandedProjectOld) *apimodels.ExpandedProject {
	//if project has no credentials, or has credentials in the newest format
	if project.GitRemoteURI == "" {
		return nil
	}

	newProject := apimodels.ExpandedProject{
		CreationDate:     project.CreationDate,
		LastEventContext: project.LastEventContext,
		ProjectName:      project.ProjectName,
		Shipyard:         project.Shipyard,
		ShipyardVersion:  project.ShipyardVersion,
		Stages:           project.Stages,
		GitCredentials: &apimodels.GitAuthCredentialsSecure{
			RemoteURL: project.GitRemoteURI,
			User:      project.GitUser,
		},
	}

	if strings.HasPrefix(project.GitRemoteURI, "http") {
		newProject.GitCredentials.HttpsAuth = &apimodels.HttpsGitAuthSecure{
			InsecureSkipTLS: project.InsecureSkipTLS,
		}

		if project.GitProxyURL != "" {
			newProject.GitCredentials.HttpsAuth.Proxy = &apimodels.ProxyGitAuthSecure{
				Scheme: project.GitProxyScheme,
				URL:    project.GitProxyURL,
				User:   project.GitProxyUser,
			}
		}
	}

	return &newProject
}

func (m *mongoDBProjectCredentialsRepo) UpdateProject(project *models.ExpandedProjectOld) error {
	newProject := TransformGitCredentials(project)
	if newProject == nil {
		return nil
	}

	return m.ProjectRepo.UpdateProject(newProject)
}
