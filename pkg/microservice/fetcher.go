package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"io"
	"net/http"
	"./node_type.go"
)

////////////////////////////////////////////////////////////////////////////////////
// HTTP CLIENT（最简单形式，没有依赖 rest.Interface）
////////////////////////////////////////////////////////////////////////////////////

type SimpleHTTPClient struct {
	BaseURL string
	Client  *http.Client
}

func NewSimpleHTTPClient(base string) *SimpleHTTPClient {
	return &SimpleHTTPClient{
		BaseURL: base,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

////////////////////////////////////////////////////////////////////////////////////
// Fetcher — 负责调用节点相关 API
////////////////////////////////////////////////////////////////////////////////////

type Fetcher struct {
	http *SimpleHTTPClient
}

func NewFetcher(baseURL string) *Fetcher {
	return &Fetcher{
		http: NewSimpleHTTPClient(baseURL),
	}
}

////////////////////////////////////////////////////////////////////////////////////
// List: /api/v1/node（分页）
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) ListNode(ctx context.Context, opts NodeListOptions) (*NodeList, error) {
	u, _ := url.Parse(f.http.BaseURL + "/api/v1/node")
	q := u.Query()
	q.Set("pageNum", fmt.Sprintf("%d", opts.PageNum))
	q.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	if opts.Name != "" {
		q.Set("name", opts.Name)
	}
	if opts.BasicInfo {
		q.Set("basicInfo", "true")
	}
	u.RawQuery = q.Encode()

	resp, err := f.http.Client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析结构：data → NodeList
	var result struct {
		Status  int                `json:"status"`
		Message string             `json:"message"`
		Data    NodeList `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (f *Fetcher) ListContainerByNodePage(ctx context.Context, opts ListContainersByNodeOptions) (*ContainerList, error) {
    u, _ := url.Parse(f.http.BaseURL + "/api/v1/container/node")
    q := u.Query()

    q.Set("pageNum", fmt.Sprintf("%d", opts.PageNum))
    q.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
    
    for _, id := range opts.NodeIDs {
        q.Add("nodeIds[]", id)
    }

    u.RawQuery = q.Encode()

    // GET
    req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
    if err != nil {
        return nil, err
    }

    resp, err := f.http.Client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    // JSON 解包
    var result struct {
        Status  int                  `json:"status"`
        Message string               `json:"message"`
        Data    ContainerList		 `json:"data"`
    }

    if err := json.Unmarshal(body, &result); err != nil {
        return nil, err
    }

    if result.Status != 200 {
        return nil, fmt.Errorf("API error: %s", result.Message)
    }

    return &result.Data, nil
}

func (f *Fetcher) ListService(ctx context.Context, opts ListServicesOptions) (*ServiceList, error) {
	u, _ := url.Parse(f.http.BaseURL + "/api/v1/service")
	q := u.Query()
	q.Set("pageNum", fmt.Sprintf("%d", opts.PageNum))
	q.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	if opts.Name != "" {
		q.Set("name", opts.Name)
	}
	if opts.BasicInfo {
		q.Set("basicInfo", "true")
	}
	u.RawQuery = q.Encode()

	resp, err := f.http.Client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析结构：data → NodeList
	var result struct {
		Status  int                `json:"status"`
		Message string             `json:"message"`
		Data    ServiceList		`json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

////////////////////////////////////////////////////////////////////////////////////
// ListAll: 自动翻页，直到全部节点返回
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) ListAllNode(ctx context.Context, opts NodeListOptions) ([]NodeInfo, error) {
	var all []NodeInfo
	opts.PageNum = 1
	if opts.PageSize == 0 {
		opts.PageSize = 50
	}

	for {
		page, err := f.ListNode(ctx, opts)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Items...)

		if len(all) >= page.Total {
			break
		}
		opts.PageNum++
	}

	return all, nil
}

func (f *Fetcher) ListContainerByNode(ctx context.Context, opts ListContainersByNodeOptions) ([]ContainerInfo, error) {
    var all []ContainerInfo

    // 初始化分页
    if opts.PageNum == 0 {
        opts.PageNum = 1
    }
    if opts.PageSize == 0 {
        opts.PageSize = 50
    }

    for {
        page, err := f.ListContainerByNodePage(ctx, opts)
        if err != nil {
            return nil, err
        }

        all = append(all, page.Items...)

        if len(all) >= page.Total {
            break
        }

        opts.PageNum++
    }

    return all, nil
}

func (f *Fetcher) ListAllService(ctx context.Context, opts ListServicesOptions) ([]ServiceList, error) {
	var all []ServiceList
	opts.PageNum = 1
	if opts.PageSize == 0 {
		opts.PageSize = 50
	}

	for {
		page, err := f.ListService(ctx, opts)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Items...)

		if len(all) >= page.Total {
			break
		}
		opts.PageNum++
	}

	return all, nil
}
////////////////////////////////////////////////////////////////////////////////////
// ListStatus: /api/v1/node/status?ids[]=...
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) ListNodeStatus(ctx context.Context, nodeIDs []string) ([]NodeStatus, error) {
	u, _ := url.Parse(f.http.BaseURL + "/api/v1/node/status")
	q := u.Query()
	for _, id := range nodeIDs {
		q.Add("ids[]", id)
	}
	u.RawQuery = q.Encode()

	resp, err := f.http.Client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// data → { nodes: [...] }
	var result struct {
		Status  int                         `json:"status"`
		Message string                      `json:"message"`
		Data    clientset.NodeStatusResponse `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Data.Nodes, nil
}

func (f *Fetcher) ListContainerStatus(ctx context.Context, containerIDs []string) ([]ContainerInfo, error) {
	var results []ContainerInfo

	for _, id := range containerIDs {

		// 构建 URL
		u, _ := url.Parse(f.http.BaseURL + "/api/v1/container/" + id)

		// 发起 GET 请求
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build request for id=%s: %w", id, err)
		}

		resp, err := f.http.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed for id=%s: %w", id, err)
		}
		defer resp.Body.Close()

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body failed for id=%s: %w", id, err)
		}

		// HTTP code check
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("container %s request failed: status=%d body=%s",
				id, resp.StatusCode, string(body))
		}

		// 解析 JSON → ContainerInfo
		var result struct {
			Status  int           `json:"status"`
			Message string        `json:"message"`
			Data    ContainerInfo `json:"data"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("json decode failed for id=%s: %w", id, err)
		}

		// 将解析结果加入结果数组
		results = append(results, result.Data)
	}

	return results, nil
}

func (f *Fetcher) ListServiceStatus(ctx context.Context, serviceIDs []string) ([]ServiceGet, error) {
	var results []ServiceGet

	for _, id := range serviceIDs {

		// 构建 URL
		u, _ := url.Parse(f.http.BaseURL + "/api/v1/service/" + id)

		// 发起 GET 请求
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build request for id=%s: %w", id, err)
		}

		resp, err := f.http.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed for id=%s: %w", id, err)
		}
		defer resp.Body.Close()

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body failed for id=%s: %w", id, err)
		}

		// HTTP code check
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("service %s request failed: status=%d body=%s",
				id, resp.StatusCode, string(body))
		}

		// 解析 JSON → ContainerInfo
		var result struct {
			Status  int           `json:"status"`
			Message string        `json:"message"`
			Data    ServiceGet `json:"data"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("json decode failed for id=%s: %w", id, err)
		}

		// 将解析结果加入结果数组
		results = append(results, result.Data)
	}

	return results, nil
}
////////////////////////////////////////////////////////////////////////////////////
// FetchAllNodeStatus: 综合流程：
// 1）ListAll 获取所有节点 ID
// 2）ListStatus 查询所有节点实时运行状态
// 只输出节点状态
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) FetchAllNodeStatus(ctx context.Context) ([]NodeStatus, error) {
	// 第一步：获取所有节点
	allNodes, err := f.ListAllNode(ctx, NodeListOptions{
		PageSize: 50,
	})
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, n := range allNodes {
		ids = append(ids, n.ID)
	}

	// 第二步：获取所有节点状态
	statusList, err := f.ListNodeStatus(ctx, ids)
	if err != nil {
		return nil, err
	}

	return statusList, nil
}

func (f *Fetcher) FetchAllContainerStatus(ctx context.Context) ([]ContainerInfo, error) {
	// 第一步：获取所有节点
	allNodes, err := f.ListAllNode(ctx, NodeListOptions{
		PageSize: 50,
	})
	if err != nil {
		return nil, err
	}

	var Nodeids []string
	for _, n := range allNodes {
		Nodeids = append(Nodeids, n.ID)
	}

	// 第二步：获取所有节点下的容器
	allContainers, err := f.ListContainerByNode(ctx, ListContainersByNodeOptions{
		PageSize: 50,
		NodeIDs: Nodeids,
	})
	if err != nil {
		return nil, err
	}

	var Containerids []string
	for _, n := range allContainers {
		Containerids = append(Containerids, n.ID)
	}
	// 第三步：获取所有容器状态
	statusList, err := f.ListContainerStatus(ctx, Containerids)
	if err != nil {
		return nil, err
	}

	return statusList, nil
}

func (f *Fetcher) FetchAllServiceStatus(ctx context.Context) ([]ServiceGet, error) {
	// 第一步：获取所有服务
	allServices, err := f.ListAllService(ctx, ListServicesOptions{
		PageSize: 50,
	})
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, n := range allServices {
		ids = append(ids, n.ID)
	}

	// 第二步：获取所有节点状态
	statusList, err := f.ListServiceStatus(ctx, ids)
	if err != nil {
		return nil, err
	}

	return statusList, nil
}
////////////////////////////////////////////////////////////////////////////////////
// main：测试用，打印所有节点状态
////////////////////////////////////////////////////////////////////////////////////
// 将[]ServiceGet,[]ContainerInfo,[]NodeStatus->extractor.go解析
func main() {
	ctx := context.Background()

	fetcher := NewFetcher("http://localhost:3001")

	statusList, err := fetcher.FetchAllNodeStatus(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("===== 所有节点状态 =====")
	for _, st := range statusList {
		fmt.Printf("NodeID=%s Status=%s CPU=%.2f%% MemFree=%d ContainerRunning=%d\n",
			st.ID,
			st.Status,
			st.CPUUsage.Total,
			st.MemoryFree,
			st.ContainerRunning,
		)
	}


	statusList, err := fetcher.FetchAllContainerStatus(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("===== 所有容器状态 =====")
	for _, st := range statusList {
		fmt.Printf("ContainerID=%s Status=%s CPU=%.2f%% MemoryUsage=%d SizeUsage=%d\n",
			st.ID,
			st.Status,
			st.CPUUsage.Total,
			st.MemoryUsage,
			st.SizeUsage,
		)
	}

	statusList, err := fetcher.FetchAllServiceStatus(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("===== 所有服务状态 =====")
	for _, st := range statusList {
		fmt.Printf("ServiceID=%s Status=%s Healthy=%.d Factor=%d InstanceActive=%d\n",
			st.ID,
			st.Status,
			st.Healthy,
			st.Factor,
			st.InstanceActive,
		)
	}
}
