package servicediscovery

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockWriter struct {
	mock.Mock
}

func (w *MockWriter) Write(p []byte) (n int, err error) {
	args := w.Called(p)
	return args.Int(0), args.Error(1)
}

type MockAPIClient struct {
	err      error
	response GraphQLResponse
}

func (c MockAPIClient) Query(query string, response interface{}) error {
	if c.err != nil {
		return c.err
	}
	*response.(*GraphQLResponse) = c.response
	return nil
}

func newGraphQLResponse(checks []Healthcheck) GraphQLResponse {
	return GraphQLResponse{
		Data: Data{
			Healthchecks: checks,
		},
	}
}

func TestWrite(t *testing.T) {
	testCases := map[string]struct {
		bizOpsResponse GraphQLResponse
		expectedWrite  string
		bizOpsError    error
		expectedErr    error
		writerErr      error
	}{
		"successful biz-ops response should write to JSON": {
			bizOpsResponse: newGraphQLResponse([]Healthcheck{
				Healthcheck{
					ID:     "someSystemCode.check",
					URL:    "https://url.com",
					IsLive: true,
					Systems: []System{
						System{
							SystemCode: "someSystemCode",
						},
					},
				},
				Healthcheck{
					ID:     "system2.check",
					URL:    "https://url2.com",
					IsLive: false,
					Systems: []System{
						System{
							SystemCode: "someSystemCode2",
						},
					},
				}}),
			expectedWrite: `[
				{
				    "targets": [
						"https://url.com"
					],
					"labels": {
						"observe": "yes"
					}
				},
				{
					"targets": [
						"https://url2.com"
					],
					"labels": {
						"observe": "no"
					}
				}
			]`,
			bizOpsError: nil,
			expectedErr: nil,
		},
		"invalid healthcheck URL in biz-ops response should be skipped": {
			bizOpsResponse: newGraphQLResponse([]Healthcheck{
				Healthcheck{
					ID:     "someSystemCode.check",
					URL:    "not_a_url",
					IsLive: true,
					Systems: []System{
						System{
							SystemCode: "someSystemCode",
						},
					},
				},
				Healthcheck{
					ID:     "system2.check",
					URL:    "https://url2.com",
					IsLive: false,
					Systems: []System{
						System{
							SystemCode: "someSystemCode2",
						},
					},
				}}),
			expectedWrite: `[
				{
					"targets": [],
					"labels": {
						"observe": "yes"
					}
				},
				{
					"targets": [
						"https://url2.com"
					],
					"labels": {
						"observe": "no"
					}
				}
			]`,
			bizOpsError: nil,
			expectedErr: nil,
		},
		"all invalid healthchecks biz-ops response should error": {
			bizOpsResponse: newGraphQLResponse([]Healthcheck{
				Healthcheck{
					ID:     "someSystemCode.check",
					URL:    "not_a_url.com",
					IsLive: true,
					Systems: []System{
						System{
							SystemCode: "someSystemCode",
						},
					},
				}}),
			expectedWrite: "",
			bizOpsError:   nil,
			expectedErr:   errors.New("processed healthchecks were empty"),
		},
		"empty healthchecks biz-ops response should return an error": {
			bizOpsResponse: newGraphQLResponse([]Healthcheck{}),
			expectedWrite:  "",
			bizOpsError:    nil,
			expectedErr:    errors.New("returned healthchecks were empty"),
		},
		"with graphql error should return an error": {
			bizOpsResponse: GraphQLResponse{},
			expectedWrite:  "",
			bizOpsError:    errors.New("biz-ops API call failed"),
			expectedErr:    errors.New("biz-ops API call failed"),
		},
		"with writer error should return an error": {
			bizOpsResponse: newGraphQLResponse([]Healthcheck{
				Healthcheck{
					ID:     "someSystemCode.check",
					URL:    "https://url.com",
					IsLive: true,
					Systems: []System{
						System{
							SystemCode: "someSystemCode",
						},
					},
				},
			}),
			expectedWrite: "",
			bizOpsError:   nil,
			expectedErr:   errors.New("Write failed"),
			writerErr:     errors.New("Write failed"),
		},
	}

	for name, test := range testCases {
		t.Run(fmt.Sprintf("Running test case: %s", name), func(t *testing.T) {

			writer := MockWriter{}
			writer.On("Write", mock.Anything).Return(len(test.expectedWrite), test.writerErr)

			apiClient := MockAPIClient{
				err:      test.bizOpsError,
				response: test.bizOpsResponse,
			}

			serviceDiscovery := BizOps{
				Writer:    &writer,
				ApiClient: &apiClient,
			}

			err := serviceDiscovery.Write()

			if test.expectedErr != nil {
				assert.Equalf(t, test.expectedErr, err, "Expected error %s", test.expectedErr)
			} else {
				assert.NoErrorf(t, err, "Error not expected")
			}

			if test.expectedWrite != "" {
				require.Equalf(t, 1, len(writer.Calls), "Expected the number of calls to Write to equal 1")
				writtenString := string(writer.Calls[0].Arguments.Get(0).([]byte))
				assert.JSONEqf(t, test.expectedWrite, writtenString, "JSON created was not as expected")
				writer.AssertExpectations(t)
			} else {
				writer.AssertNotCalled(t, "Write")
			}
		})
	}
}
