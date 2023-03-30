}

func (c *Client) CancelDeployment(ctx context.Context, req CancelDeploymentRequest) error {
	return c.post(ctx, "/deployments/cancel", req, nil)
}

func (c *Client) GetDeploymentLogs(ctx context.Context, deploymentID string, prevToken string) (res GetDeploymentLogsResponse, err error) {
	q := url.Values{
		"id": []string{deploymentID},
	}
	if logger.EnableDebug {
		q.Set("level", "debug")
	}
	if prevToken != "" {
		q.Set("prevToken", prevToken)
	}
	err = c.get(ctx, encodeQueryString("/deployments/getLogs", q), &res)
	return
}

func (c *Client) ListResources(ctx context.Context, envSlug string) (res libapi.ListResourcesResponse, err error) {
	err = c.get(ctx, encodeQueryString("/resources/list", url.Values{
		"envSlug": []string{envSlug},
	}), &res)
	return
}

func (c *Client) ListResourceMetadata(ctx context.Context) (res libapi.ListResourceMetadataResponse, err error) {
	err = c.get(ctx, "/resources/listMetadata", &res)
	return
}

func (c *Client) GetResource(ctx context.Context, req GetResourceRequest) (res libapi.GetResourceResponse, err error) {
	err = c.get(ctx, encodeQueryString("/resources/get", url.Values{
		"id":                   []string{req.ID},
		"slug":                 []string{req.Slug},
		"envSlug":              []string{req.EnvSlug},
		"includeSensitiveData": []string{strconv.FormatBool(req.IncludeSensitiveData)},
	}), &res)
	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, libapi.ResourceMissingError{
			AppURL: c.AppURL().String(),
			Slug:   req.Slug,
		}
	}
	return
}

func (c *Client) GetEnv(ctx context.Context, envSlug string) (res libapi.Env, err error) {
	err = c.get(ctx, encodeQueryString("/envs/get", url.Values{
		"slug": []string{envSlug},
	}), &res)
	return
}

func (c *Client) ListEnvs(ctx context.Context) (res ListEnvsResponse, err error) {
	err = c.get(ctx, "/envs/list", &res)
	return
}

func (c *Client) EvaluateTemplate(ctx context.Context, req libapi.EvaluateTemplateRequest) (res libapi.EvaluateTemplateResponse, err error) {
	err = c.post(ctx, "/templates/evaluate", req, &res)
	return
}

func (c *Client) GetPermissions(ctx context.Context, taskSlug string, actions []string) (res GetPermissionsResponse, err error) {
	err = c.get(ctx, encodeQueryString("/permissions/get", url.Values{
		"task_slug": []string{taskSlug},
		"actions":   actions,
	}), &res)
	return
}

func (c *Client) CreateUpload(ctx context.Context, req libapi.CreateUploadRequest) (res libapi.CreateUploadResponse, err error) {
	err = c.post(ctx, "/uploads/create", req, &res)
	return
}

func (c *Client) GetUpload(ctx context.Context, uploadID string) (res libapi.GetUploadResponse, err error) {
	err = c.get(ctx, encodeQueryString("/uploads/get", url.Values{
		"id": []string{uploadID},
	}), &res)
	return
}

func (c *Client) GenerateSignedURLs(ctx context.Context, envSlug string) (res GenerateSignedURLsResponse, err error) {
	err = c.get(ctx, encodeQueryString("/uploads/generateSignedURLs", url.Values{
		"envSlug": []string{envSlug},
	}), &res)
	return
}

func (c *Client) GetWebHost(ctx context.Context) (webHost string, err error) {
	err = c.get(ctx, "/hosts/web", &webHost)
	return
}

func (c *Client) GetUser(ctx context.Context, userID string) (res GetUserResponse, err error) {
	err = c.get(ctx, encodeQueryString("/users/get", url.Values{
		"userID": []string{userID},
	}), &res)
	return
}

func (c *Client) GetTunnelToken(ctx context.Context) (res GetTunnelTokenResponse, err error) {
	err = c.get(ctx, "/studio/tunnelToken/get", &res)
	return
}

func (c *Client) SetDevSecret(ctx context.Context, token string) (err error) {
	return c.post(ctx, "/studio/tunnelToken/setDevSecret", &SetDevSecretRequest{
		Token: token,
	}, nil)
}

func (c *Client) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (res CreateSandboxResponse, err error) {
	err = c.post(ctx, "/studio/createSandbox", req, &res)
	return
}

func (c *Client) AutopilotComplete(ctx context.Context, req AutopilotCompleteRequest) (res AutopilotCompleteResponse, err error) {
	err = c.post(ctx, "/autopilot/complete", req, &res)
	return
}

func (c *Client) headers() (map[string]string, error) {
	headers := map[string]string{}
	if c.Token() != "" {
		headers["X-Airplane-Token"] = c.Token()
	} else if c.apiKey != "" {
		headers["X-Airplane-API-Key"] = c.apiKey
		if c.teamID == "" {
			return nil, errors.New("team ID is missing")
		}
		headers["X-Team-ID"] = c.teamID
	} else {
		return nil, errors.Errorf("authentication is missing: %s", c.apiKey)
	}

	if c.tunnelToken != nil {
		headers["X-Airplane-Dev-Token"] = *c.tunnelToken
	}

	return headers, nil
}

func (c *Client) get(ctx context.Context, path string, reply interface{}) error {
	headers, err := c.headers()
	if err != nil {
		return err
	}

	pathname := "/v0" + path
	url := c.scheme() + c.Host() + pathname
	err = c.http.GetJSON(ctx, url, reply, libhttp.ReqOpts{
		Headers: headers,
	})
	if err != nil {
		logger.Debug("GET %s: request failed: %v", pathname, err)
		return err
	}

	return nil
}

func (c *Client) post(ctx context.Context, path string, payload, reply interface{}) error {
	headers, err := c.headers()
	if err != nil {
		return err
	}

	pathname := "/v0" + path
	url := c.scheme() + c.Host() + pathname
	err = c.http.PostJSON(ctx, url, payload, reply, libhttp.ReqOpts{
		Headers: headers,
	})
	if err != nil {
		logger.Debug("POST %s: request failed: %v", pathname, err)
		return err
	}

	return nil
}

// Host returns the configured endpoint.
func (c *Client) Host() string {
	if c.host != "" {
		return c.host
	}
	return DefaultAPIHost
}

func (c *Client) SetHost(host string) {
	c.host = host
}

var httpHosts = []string{
	"localhost",
	"127.0.0.1",
	"host.docker.internal",
	"172.17.0.1", // Docker for linux
	"api",
}

func (c *Client) scheme() string {
	if c.host == DefaultAPIHost {
		return "https://"
	}

	host := c.host
	// If the host didn't come with a scheme, force a "//" in front of it.
	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("//%s", host)
	}
	u, err := url.Parse(host)
	if err != nil {
		return "https://"
	}

	for _, httpHost := range httpHosts {
		if u.Hostname() == httpHost {
			return "http://"
		}
	}

	return "https://"
}

// encodeURL is a helper for encoding a set of query parameters onto a URL.
//
// If a query parameter is an empty string, it will be excluded from the
// encoded query string.
func encodeQueryString(path string, params url.Values) string {
	updatedParams := url.Values{}
	for k, v := range params {
		// Remove any query parameters
		if len(v) > 1 || (len(v) == 1 && len(v[0]) > 0) {
			updatedParams[k] = v
		}
	}

	if len(updatedParams) == 0 {
		return path
	}

	return path + "?" + updatedParams.Encode()
}
