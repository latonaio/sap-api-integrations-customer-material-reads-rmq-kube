package sap_api_caller

import (
	"fmt"
	"io/ioutil"
	"net/http"
	sap_api_output_formatter "sap-api-integrations-customer-material-reads-rmq-kube/SAP_API_Output_Formatter"
	"strings"
	"sync"

	"github.com/latonaio/golang-logging-library-for-sap/logger"
	"golang.org/x/xerrors"
)

type RMQOutputter interface {
	Send(sendQueue string, payload map[string]interface{}) error
}

type SAPAPICaller struct {
	baseURL      string
	apiKey       string
	outputQueues []string
	outputter    RMQOutputter
	log          *logger.Logger
}

func NewSAPAPICaller(baseUrl string, outputQueueTo []string, outputter RMQOutputter, l *logger.Logger) *SAPAPICaller {
	return &SAPAPICaller{
		baseURL:      baseUrl,
		apiKey:       GetApiKey(),
		outputQueues: outputQueueTo,
		outputter:    outputter,
		log:          l,
	}
}

func (c *SAPAPICaller) AsyncGetCustomerMaterial(salesOrganization, distributionChannel, customer, material string, accepter []string) {
	wg := &sync.WaitGroup{}
	wg.Add(len(accepter))
	for _, fn := range accepter {
		switch fn {
		case "CustomerMaterial":
			func() {
				c.CustomerMaterial(salesOrganization, distributionChannel, customer, material)
				wg.Done()
			}()
		default:
			wg.Done()
		}
	}
	wg.Wait()
}

func (c *SAPAPICaller) CustomerMaterial(salesOrganization, distributionChannel, customer, material string) {
	customerMaterialData, err := c.callCustomerMaterialSrvAPIRequirementCustomerMaterial("A_CustomerMaterial", salesOrganization, distributionChannel, customer, material)
	if err != nil {
		c.log.Error(err)
		return
	}
	err = c.outputter.Send(c.outputQueues[0], map[string]interface{}{"message": customerMaterialData, "function": "CustomerMaterial"})
	if err != nil {
		c.log.Error(err)
		return
	}
	c.log.Info(customerMaterialData)
}

func (c *SAPAPICaller) callCustomerMaterialSrvAPIRequirementCustomerMaterial(api, salesOrganization, distributionChannel, customer, material string) ([]sap_api_output_formatter.CustomerMaterial, error) {
	url := strings.Join([]string{c.baseURL, "API_CUSTOMER_MATERIAL_SRV", api}, "/")
	req, _ := http.NewRequest("GET", url, nil)

	c.setHeaderAPIKeyAccept(req)
	c.getQueryWithCustomerMaterial(req, salesOrganization, distributionChannel, customer, material)

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil, xerrors.Errorf("API request error: %w", err)
	}
	defer resp.Body.Close()

	byteArray, _ := ioutil.ReadAll(resp.Body)
	data, err := sap_api_output_formatter.ConvertToCustomerMaterial(byteArray, c.log)
	if err != nil {
		return nil, xerrors.Errorf("convert error: %w", err)
	}
	return data, nil
}

func (c *SAPAPICaller) setHeaderAPIKeyAccept(req *http.Request) {
	req.Header.Set("APIKey", c.apiKey)
	req.Header.Set("Accept", "application/json")
}

func (c *SAPAPICaller) getQueryWithCustomerMaterial(req *http.Request, salesOrganization, distributionChannel, customer, material string) {
	params := req.URL.Query()
	params.Add("$filter", fmt.Sprintf("SalesOrganization eq '%s' and DistributionChannel eq '%s' and Customer eq '%s' and Material eq '%s'", salesOrganization, distributionChannel, customer, material))
	req.URL.RawQuery = params.Encode()
}
