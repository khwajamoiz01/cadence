// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/uber/cadence/tools/common/flag"

	"github.com/urfave/cli"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/types"
)

var (
	gracefulFailoverType = "grace"
)

type (
	domainCLIImpl struct {
		// used when making RPC call to frontend service
		frontendClient frontend.Client

		// act as admin to modify domain in DB directly
		domainHandler domain.Handler
	}
)

// newDomainCLI creates a domain CLI
func newDomainCLI(
	c *cli.Context,
	isAdminMode bool,
) *domainCLIImpl {

	var frontendClient frontend.Client
	var domainHandler domain.Handler
	if !isAdminMode {
		frontendClient = initializeFrontendClient(c)
	} else {
		domainHandler = initializeAdminDomainHandler(c)
	}
	return &domainCLIImpl{
		frontendClient: frontendClient,
		domainHandler:  domainHandler,
	}
}

// RegisterDomain register a domain
func (d *domainCLIImpl) RegisterDomain(c *cli.Context) {
	domainName := getRequiredGlobalOption(c, FlagDomain)

	description := c.String(FlagDescription)
	ownerEmail := c.String(FlagOwnerEmail)
	retentionDays := defaultDomainRetentionDays

	if c.IsSet(FlagRetentionDays) {
		retentionDays = c.Int(FlagRetentionDays)
	}
	securityToken := c.String(FlagSecurityToken)
	var err error

	isGlobalDomain := true
	if c.IsSet(FlagIsGlobalDomain) {
		isGlobalDomain, err = strconv.ParseBool(c.String(FlagIsGlobalDomain))
		if err != nil {
			ErrorAndExit(fmt.Sprintf("Option %s format is invalid.", FlagIsGlobalDomain), err)
		}
	}

	var domainData *flag.StringMap
	if c.IsSet(FlagDomainData) {
		domainData = c.Generic(FlagDomainData).(*flag.StringMap)
	}
	if len(requiredDomainDataKeys) > 0 {
		err = checkRequiredDomainDataKVs(domainData.Value())
		if err != nil {
			ErrorAndExit("Domain data missed required data.", err)
		}
	}

	activeClusterName := ""
	if c.IsSet(FlagActiveClusterName) {
		activeClusterName = c.String(FlagActiveClusterName)
	}

	var clusters []*types.ClusterReplicationConfiguration
	if c.IsSet(FlagClusters) {
		clusterStr := c.String(FlagClusters)
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: clusterStr,
		})
		for _, clusterStr := range c.Args() {
			clusters = append(clusters, &types.ClusterReplicationConfiguration{
				ClusterName: clusterStr,
			})
		}
	}

	request := &types.RegisterDomainRequest{
		Name:                                   domainName,
		Description:                            description,
		OwnerEmail:                             ownerEmail,
		Data:                                   domainData.Value(),
		WorkflowExecutionRetentionPeriodInDays: int32(retentionDays),
		Clusters:                               clusters,
		ActiveClusterName:                      activeClusterName,
		SecurityToken:                          securityToken,
		HistoryArchivalStatus:                  archivalStatus(c, FlagHistoryArchivalStatus),
		HistoryArchivalURI:                     c.String(FlagHistoryArchivalURI),
		VisibilityArchivalStatus:               archivalStatus(c, FlagVisibilityArchivalStatus),
		VisibilityArchivalURI:                  c.String(FlagVisibilityArchivalURI),
		IsGlobalDomain:                         isGlobalDomain,
	}

	ctx, cancel := newContext(c)
	defer cancel()
	err = d.registerDomain(ctx, request)
	if err != nil {
		if _, ok := err.(*types.DomainAlreadyExistsError); !ok {
			ErrorAndExit("Register Domain operation failed.", err)
		} else {
			ErrorAndExit(fmt.Sprintf("Domain %s already registered.", domainName), err)
		}
	} else {
		fmt.Printf("Domain %s successfully registered.\n", domainName)
	}
}

// UpdateDomain updates a domain
func (d *domainCLIImpl) UpdateDomain(c *cli.Context) {
	domainName := getRequiredGlobalOption(c, FlagDomain)

	var updateRequest *types.UpdateDomainRequest
	ctx, cancel := newContext(c)
	defer cancel()

	if c.IsSet(FlagActiveClusterName) {
		activeCluster := c.String(FlagActiveClusterName)
		fmt.Printf("Will set active cluster name to: %s, other flag will be omitted.\n", activeCluster)

		var failoverTimeout *int32
		if c.String(FlagFailoverType) == gracefulFailoverType {
			timeout := int32(c.Int(FlagFailoverTimeout))
			failoverTimeout = &timeout
		}

		updateRequest = &types.UpdateDomainRequest{
			Name:                     domainName,
			ActiveClusterName:        common.StringPtr(activeCluster),
			FailoverTimeoutInSeconds: failoverTimeout,
		}
	} else {
		resp, err := d.describeDomain(ctx, &types.DescribeDomainRequest{
			Name: common.StringPtr(domainName),
		})
		if err != nil {
			if _, ok := err.(*types.EntityNotExistsError); !ok {
				ErrorAndExit("Operation UpdateDomain failed.", err)
			} else {
				ErrorAndExit(fmt.Sprintf("Domain %s does not exist.", domainName), err)
			}
			return
		}

		description := resp.DomainInfo.GetDescription()
		ownerEmail := resp.DomainInfo.GetOwnerEmail()
		retentionDays := resp.Configuration.GetWorkflowExecutionRetentionPeriodInDays()
		emitMetric := resp.Configuration.GetEmitMetric()
		var clusters []*types.ClusterReplicationConfiguration

		if c.IsSet(FlagDescription) {
			description = c.String(FlagDescription)
		}
		if c.IsSet(FlagOwnerEmail) {
			ownerEmail = c.String(FlagOwnerEmail)
		}
		var domainData *flag.StringMap
		if c.IsSet(FlagDomainData) {
			domainData = c.Generic(FlagDomainData).(*flag.StringMap)
		}
		if c.IsSet(FlagRetentionDays) {
			retentionDays = int32(c.Int(FlagRetentionDays))
		}
		if c.IsSet(FlagClusters) {
			clusterStr := c.String(FlagClusters)
			clusters = append(clusters, &types.ClusterReplicationConfiguration{
				ClusterName: clusterStr,
			})
			for _, clusterStr := range c.Args() {
				clusters = append(clusters, &types.ClusterReplicationConfiguration{
					ClusterName: clusterStr,
				})
			}
		}

		var binBinaries *types.BadBinaries
		if c.IsSet(FlagAddBadBinary) {
			if !c.IsSet(FlagReason) {
				ErrorAndExit("Must provide a reason.", nil)
			}
			binChecksum := c.String(FlagAddBadBinary)
			reason := c.String(FlagReason)
			operator := getCurrentUserFromEnv()
			binBinaries = &types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{
					binChecksum: {
						Reason:   reason,
						Operator: operator,
					},
				},
			}
		}

		var badBinaryToDelete *string
		if c.IsSet(FlagRemoveBadBinary) {
			badBinaryToDelete = common.StringPtr(c.String(FlagRemoveBadBinary))
		}

		updateRequest = &types.UpdateDomainRequest{
			Name:                                   domainName,
			Description:                            common.StringPtr(description),
			OwnerEmail:                             common.StringPtr(ownerEmail),
			Data:                                   domainData.Value(),
			WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(retentionDays),
			EmitMetric:                             common.BoolPtr(emitMetric),
			HistoryArchivalStatus:                  archivalStatus(c, FlagHistoryArchivalStatus),
			HistoryArchivalURI:                     common.StringPtr(c.String(FlagHistoryArchivalURI)),
			VisibilityArchivalStatus:               archivalStatus(c, FlagVisibilityArchivalStatus),
			VisibilityArchivalURI:                  common.StringPtr(c.String(FlagVisibilityArchivalURI)),
			BadBinaries:                            binBinaries,
			Clusters:                               clusters,
			DeleteBadBinary:                        badBinaryToDelete,
		}
	}

	securityToken := c.String(FlagSecurityToken)
	updateRequest.SecurityToken = securityToken
	_, err := d.updateDomain(ctx, updateRequest)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			ErrorAndExit("Operation UpdateDomain failed.", err)
		} else {
			ErrorAndExit(fmt.Sprintf("Domain %s does not exist.", domainName), err)
		}
	} else {
		fmt.Printf("Domain %s successfully updated.\n", domainName)
	}
}

func (d *domainCLIImpl) DeprecateDomain(c *cli.Context) {
	domainName := getRequiredGlobalOption(c, FlagDomain)
	securityToken := c.String(FlagSecurityToken)
	force := c.Bool(FlagForce)

	ctx, cancel := newContext(c)
	defer cancel()

	if !force {
		// check if there is any workflow in this domain, if exists, do not deprecate
		wfs, _ := listClosedWorkflow(getWorkflowClient(c), 1, 0, time.Now().UnixNano(), domainName, "", "", workflowStatusNotSet, c)(nil)
		if len(wfs) > 0 {
			ErrorAndExit("Operation DeprecateDomain failed.", errors.New("workflow history not cleared in this domain"))
			return
		}
		wfs, _ = listOpenWorkflow(getWorkflowClient(c), 1, 0, time.Now().UnixNano(), domainName, "", "", c)(nil)
		if len(wfs) > 0 {
			ErrorAndExit("Operation DeprecateDomain failed.", errors.New("workflow still running in this domain"))
			return
		}
	}
	err := d.deprecateDomain(ctx, &types.DeprecateDomainRequest{
		Name:          domainName,
		SecurityToken: securityToken,
	})
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			ErrorAndExit("Operation DeprecateDomain failed.", err)
		} else {
			ErrorAndExit(fmt.Sprintf("Domain %s does not exist.", domainName), err)
		}
	} else {
		fmt.Printf("Domain %s successfully deprecated.\n", domainName)
	}
}

// FailoverDomains is used for managed failover all domains with domain data IsManagedByCadence=true
func (d *domainCLIImpl) FailoverDomains(c *cli.Context) {
	// ask user for confirmation
	prompt("You are trying to failover all managed domains, continue? Y/N")
	d.failoverDomains(c)
}

// return succeed and failed domains for testing purpose
func (d *domainCLIImpl) failoverDomains(c *cli.Context) ([]string, []string) {
	targetCluster := getRequiredOption(c, FlagActiveClusterName)
	domains := d.getAllDomains(c)
	shouldFailover := func(domain *types.DescribeDomainResponse) bool {
		isDomainNotActiveInTargetCluster := domain.ReplicationConfiguration.GetActiveClusterName() != targetCluster
		return isDomainNotActiveInTargetCluster && isDomainFailoverManagedByCadence(domain)
	}
	var succeedDomains []string
	var failedDomains []string
	for _, domain := range domains {
		if shouldFailover(domain) {
			domainName := domain.GetDomainInfo().GetName()
			err := d.failover(c, domainName, targetCluster)
			if err != nil {
				printError(fmt.Sprintf("Failed failover domain: %s\n", domainName), err)
				failedDomains = append(failedDomains, domainName)
			} else {
				fmt.Printf("Success failover domain: %s\n", domainName)
				succeedDomains = append(succeedDomains, domainName)
			}
		}
	}
	fmt.Printf("Succeed %d: %v\n", len(succeedDomains), succeedDomains)
	fmt.Printf("Failed  %d: %v\n", len(failedDomains), failedDomains)
	return succeedDomains, failedDomains
}

func (d *domainCLIImpl) getAllDomains(c *cli.Context) []*types.DescribeDomainResponse {
	var res []*types.DescribeDomainResponse
	pagesize := int32(200)
	var token []byte
	ctx, cancel := newContext(c)
	defer cancel()
	for more := true; more; more = len(token) > 0 {
		listRequest := &types.ListDomainsRequest{
			PageSize:      pagesize,
			NextPageToken: token,
		}
		listResp, err := d.listDomains(ctx, listRequest)
		if err != nil {
			ErrorAndExit("Error when list domains info", err)
		}
		token = listResp.GetNextPageToken()
		res = append(res, listResp.GetDomains()...)
	}
	return res
}

func isDomainFailoverManagedByCadence(domain *types.DescribeDomainResponse) bool {
	domainData := domain.DomainInfo.GetData()
	return strings.ToLower(strings.TrimSpace(domainData[common.DomainDataKeyForManagedFailover])) == "true"
}

func (d *domainCLIImpl) failover(c *cli.Context, domainName string, targetCluster string) error {
	updateRequest := &types.UpdateDomainRequest{
		Name:              domainName,
		ActiveClusterName: common.StringPtr(targetCluster),
	}
	ctx, cancel := newContext(c)
	defer cancel()
	_, err := d.updateDomain(ctx, updateRequest)
	return err
}

// DescribeDomain updates a domain
func (d *domainCLIImpl) DescribeDomain(c *cli.Context) {
	domainName := c.GlobalString(FlagDomain)
	domainID := c.String(FlagDomainID)
	printJSON := c.Bool(FlagPrintJSON)

	request := types.DescribeDomainRequest{}
	if domainID != "" {
		request.UUID = &domainID
	}
	if domainName != "" {
		request.Name = &domainName
	}
	if domainID == "" && domainName == "" {
		ErrorAndExit("At least domainID or domainName must be provided.", nil)
	}

	ctx, cancel := newContext(c)
	defer cancel()
	resp, err := d.describeDomain(ctx, &request)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			ErrorAndExit("Operation DescribeDomain failed.", err)
		}
		ErrorAndExit(fmt.Sprintf("Domain %s does not exist.", domainName), err)
	}

	if printJSON {
		output, err := json.Marshal(resp)
		if err != nil {
			ErrorAndExit("Failed to encode domain response into JSON.", err)
		}
		fmt.Println(string(output))
		return
	}

	clusters := "N/A, Not a global domain"
	if resp.IsGlobalDomain {
		clusters = clustersToString(resp.ReplicationConfiguration.Clusters)
	}
	var formatStr = "Name: %v\nUUID: %v\nDescription: %v\nOwnerEmail: %v\nDomainData: %v\nStatus: %v\nRetentionInDays: %v\n" +
		"EmitMetrics: %v\nIsGlobal(XDC)Domain: %v\nActiveClusterName: %v\nClusters: %v\nHistoryArchivalStatus: %v\n"
	descValues := []interface{}{
		resp.DomainInfo.GetName(),
		resp.DomainInfo.GetUUID(),
		resp.DomainInfo.GetDescription(),
		resp.DomainInfo.GetOwnerEmail(),
		resp.DomainInfo.Data,
		resp.DomainInfo.GetStatus(),
		resp.Configuration.GetWorkflowExecutionRetentionPeriodInDays(),
		resp.Configuration.GetEmitMetric(),
		resp.IsGlobalDomain,
		resp.ReplicationConfiguration.GetActiveClusterName(),
		clusters,
		resp.Configuration.GetHistoryArchivalStatus().String(),
	}
	if resp.Configuration.GetHistoryArchivalURI() != "" {
		formatStr = formatStr + "HistoryArchivalURI: %v\n"
		descValues = append(descValues, resp.Configuration.GetHistoryArchivalURI())
	}
	formatStr = formatStr + "VisibilityArchivalStatus: %v\n"
	descValues = append(descValues, resp.Configuration.GetVisibilityArchivalStatus().String())
	if resp.Configuration.GetVisibilityArchivalURI() != "" {
		formatStr = formatStr + "VisibilityArchivalURI: %v\n"
		descValues = append(descValues, resp.Configuration.GetVisibilityArchivalURI())
	}
	fmt.Printf(formatStr, descValues...)
	if resp.Configuration.BadBinaries != nil {
		fmt.Println("Bad binaries to reset:")
		table := []BadBinaryRow{}
		for cs, bin := range resp.Configuration.BadBinaries.Binaries {
			table = append(table, BadBinaryRow{
				Checksum:  cs,
				Operator:  bin.GetOperator(),
				StartTime: time.Unix(0, bin.GetCreatedTimeNano()),
				Reason:    bin.GetReason(),
			})
		}
		RenderTable(os.Stdout, table, TableOptions{Color: true, Border: true, PrintDateTime: true})
	}
	if resp.GetFailoverInfo() != nil {
		info := resp.GetFailoverInfo()
		fmt.Println("Graceful failover info:")
		table := []FailoverInfoRow{{
			FailoverVersion:     info.GetFailoverVersion(),
			StartTime:           time.Unix(0, info.GetFailoverStartTimestamp()),
			ExpireTime:          time.Unix(0, info.GetFailoverExpireTimestamp()),
			CompletedShardCount: info.GetCompletedShardCount(),
			PendingShard:        info.GetPendingShards(),
		}}
		RenderTable(os.Stdout, table, TableOptions{Color: true, Border: true, PrintDateTime: true})
	}
}

type BadBinaryRow struct {
	Checksum  string    `header:"Binary Checksum"`
	Operator  string    `header:"Operator"`
	StartTime time.Time `header:"Start Time"`
	Reason    string    `header:"Reason"`
}

type FailoverInfoRow struct {
	FailoverVersion     int64     `header:"Failover Version"`
	StartTime           time.Time `header:"Start Time"`
	ExpireTime          time.Time `header:"Expire Time"`
	CompletedShardCount int32     `header:"Completed Shard Count"`
	PendingShard        []int32   `header:"Pending Shard"`
}

type DomainRow struct {
	Name                     string               `header:"Name"`
	UUID                     string               `header:"UUID"`
	DomainData               string               `header:"Domain Data"`
	Status                   types.DomainStatus   `header:"Status"`
	IsGlobal                 bool                 `header:"Is Global Domain"`
	ActiveCluster            string               `header:"Active Cluster"`
	Clusters                 string               `header:"Clusters"`
	RetentionDays            int32                `header:"Retention Days"`
	HistoryArchivalStatus    types.ArchivalStatus `header:"History Archival Status"`
	HistoryArchivalURI       string               `header:"History Archival URI"`
	VisibilityArchivalStatus types.ArchivalStatus `header:"Visibility Archival Status"`
	VisibilityArchivalURI    string               `header:"Visibility Archival URI"`
}

func newDomainRow(domain *types.DescribeDomainResponse) DomainRow {
	return DomainRow{
		Name:                     domain.DomainInfo.Name,
		UUID:                     domain.DomainInfo.UUID,
		DomainData:               mapToString(domain.DomainInfo.GetData(), ", "),
		Status:                   domain.DomainInfo.GetStatus(),
		IsGlobal:                 domain.IsGlobalDomain,
		ActiveCluster:            domain.ReplicationConfiguration.GetActiveClusterName(),
		Clusters:                 clustersToString(domain.ReplicationConfiguration.GetClusters()),
		RetentionDays:            domain.Configuration.GetWorkflowExecutionRetentionPeriodInDays(),
		HistoryArchivalStatus:    domain.Configuration.GetHistoryArchivalStatus(),
		HistoryArchivalURI:       domain.Configuration.GetHistoryArchivalURI(),
		VisibilityArchivalStatus: domain.Configuration.GetVisibilityArchivalStatus(),
		VisibilityArchivalURI:    domain.Configuration.GetVisibilityArchivalURI(),
	}
}

func domainTableOptions(c *cli.Context) TableOptions {
	printAll := c.Bool(FlagAll)
	printFull := c.Bool(FlagPrintFullyDetail)

	return TableOptions{
		Color: true,
		OptionalColumns: map[string]bool{
			"Status":                     printAll || printFull,
			"Clusters":                   printFull,
			"Retention Days":             printFull,
			"History Archival Status":    printFull,
			"History Archival URI":       printFull,
			"Visibility Archival Status": printFull,
			"Visibility Archival URI":    printFull,
		},
	}
}

func (d *domainCLIImpl) ListDomains(c *cli.Context) {
	pageSize := c.Int(FlagPageSize)
	prefix := c.String(FlagPrefix)
	printAll := c.Bool(FlagAll)
	printDeprecated := c.Bool(FlagDeprecated)
	printJSON := c.Bool(FlagPrintJSON)

	if printAll && printDeprecated {
		ErrorAndExit(fmt.Sprintf("Cannot specify %s and %s flags at the same time.", FlagAll, FlagDeprecated), nil)
	}

	domains := d.getAllDomains(c)
	var filteredDomains []*types.DescribeDomainResponse

	// Only list domains that are matching to the prefix if prefix is provided
	if len(prefix) > 0 {
		var prefixDomains []*types.DescribeDomainResponse
		for _, domain := range domains {
			if strings.Index(domain.DomainInfo.Name, prefix) == 0 {
				prefixDomains = append(prefixDomains, domain)
			}
		}
		domains = prefixDomains
	}

	if printAll {
		filteredDomains = domains
	} else {
		filteredDomains = make([]*types.DescribeDomainResponse, 0, len(domains))
		for _, domain := range domains {
			if printDeprecated && *domain.DomainInfo.Status == types.DomainStatusDeprecated {
				filteredDomains = append(filteredDomains, domain)
			} else if !printDeprecated && *domain.DomainInfo.Status == types.DomainStatusRegistered {
				filteredDomains = append(filteredDomains, domain)
			}
		}
	}

	if printJSON {
		output, err := json.Marshal(filteredDomains)
		if err != nil {
			ErrorAndExit("Failed to encode domain results into JSON.", err)
		}
		fmt.Println(string(output))
		return
	}

	table := make([]DomainRow, 0, pageSize)

	currentPageSize := 0
	for i, domain := range filteredDomains {
		table = append(table, newDomainRow(domain))
		currentPageSize++

		if currentPageSize != pageSize {
			continue
		}

		// page is full
		RenderTable(os.Stdout, table, domainTableOptions(c))
		if i == len(domains)-1 || !showNextPage() {
			return
		}
		table = make([]DomainRow, 0, pageSize)
		currentPageSize = 0
	}

	RenderTable(os.Stdout, table, domainTableOptions(c))
}

func (d *domainCLIImpl) listDomains(
	ctx context.Context,
	request *types.ListDomainsRequest,
) (*types.ListDomainsResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.ListDomains(ctx, request)
	}

	return d.domainHandler.ListDomains(ctx, request)
}

func (d *domainCLIImpl) registerDomain(
	ctx context.Context,
	request *types.RegisterDomainRequest,
) error {

	if d.frontendClient != nil {
		return d.frontendClient.RegisterDomain(ctx, request)
	}

	return d.domainHandler.RegisterDomain(ctx, request)
}

func (d *domainCLIImpl) updateDomain(
	ctx context.Context,
	request *types.UpdateDomainRequest,
) (*types.UpdateDomainResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.UpdateDomain(ctx, request)
	}

	return d.domainHandler.UpdateDomain(ctx, request)
}

func (d *domainCLIImpl) deprecateDomain(
	ctx context.Context,
	request *types.DeprecateDomainRequest,
) error {

	if d.frontendClient != nil {
		return d.frontendClient.DeprecateDomain(ctx, request)
	}

	return d.domainHandler.DeprecateDomain(ctx, request)
}

func (d *domainCLIImpl) describeDomain(
	ctx context.Context,
	request *types.DescribeDomainRequest,
) (*types.DescribeDomainResponse, error) {

	if d.frontendClient != nil {
		return d.frontendClient.DescribeDomain(ctx, request)
	}

	return d.domainHandler.DescribeDomain(ctx, request)
}

func archivalStatus(c *cli.Context, statusFlagName string) *types.ArchivalStatus {
	if c.IsSet(statusFlagName) {
		switch c.String(statusFlagName) {
		case "disabled":
			return types.ArchivalStatusDisabled.Ptr()
		case "enabled":
			return types.ArchivalStatusEnabled.Ptr()
		default:
			ErrorAndExit(fmt.Sprintf("Option %s format is invalid.", statusFlagName), errors.New("invalid status, valid values are \"disabled\" and \"enabled\""))
		}
	}
	return nil
}

func clustersToString(clusters []*types.ClusterReplicationConfiguration) string {
	var res string
	for i, cluster := range clusters {
		if i == 0 {
			res = res + cluster.GetClusterName()
		} else {
			res = res + ", " + cluster.GetClusterName()
		}
	}
	return res
}
