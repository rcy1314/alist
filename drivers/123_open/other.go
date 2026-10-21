package _123Open

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	pan123 "github.com/okatu-loli/go-123pan"
)

// otherHandler serves one args.Method of the Other entry point.
type otherHandler func(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error)

// otherHandlers exposes the parts of the open platform that have no place in
// the standard driver interface. Every handler decodes args.Data into its own
// request struct (snake_case JSON) and hands back the SDK result unchanged.
var otherHandlers = map[string]otherHandler{
	// user
	"user_info": otherUserInfo,

	// files and recycle bin
	"detail":       otherDetail,
	"infos":        otherInfos,
	"trash":        otherTrash,
	"recover":      otherRecover,
	"recover_to":   otherRecoverTo,
	"batch_rename": otherBatchRename,
	"list_v1":      otherListV1,
	"safebox_id":   otherSafeboxID,

	// share
	"share_create":      otherShareCreate,
	"share_list":        otherShareList,
	"share_update":      otherShareUpdate,
	"share_create_paid": otherShareCreatePaid,
	"share_list_paid":   otherShareListPaid,
	"share_update_paid": otherShareUpdatePaid,

	// direct link
	"direct_link_enable":              otherDirectLinkEnable,
	"direct_link_disable":             otherDirectLinkDisable,
	"direct_link_url":                 otherDirectLinkURL,
	"direct_link_refresh_cache":       otherDirectLinkRefreshCache,
	"direct_link_traffic":             otherDirectLinkTraffic,
	"direct_link_offline_logs":        otherDirectLinkOfflineLogs,
	"direct_link_ip_blacklist":        otherDirectLinkIPBlacklist,
	"direct_link_ip_blacklist_update": otherDirectLinkIPBlacklistUpdate,
	"direct_link_ip_blacklist_switch": otherDirectLinkIPBlacklistSwitch,

	// offline download
	"offline_download": otherOfflineDownload,
	"offline_process":  otherOfflineProcess,

	// transcoding space
	"transcode_folder_info":            otherTranscodeFolderInfo,
	"transcode_cloud_video_files":      otherTranscodeCloudVideoFiles,
	"transcode_space_files":            otherTranscodeSpaceFiles,
	"transcode_upload_from_cloud_disk": otherTranscodeUploadFromCloudDisk,
	"transcode_resolutions":            otherTranscodeResolutions,
	"transcode_video":                  otherTranscodeVideo,
	"transcode_records":                otherTranscodeRecords,
	"transcode_results":                otherTranscodeResults,
	"transcode_list":                   otherTranscodeList,
	"transcode_delete":                 otherTranscodeDelete,
	"transcode_download_original":      otherTranscodeDownloadOriginal,
	"transcode_download_m3u8":          otherTranscodeDownloadM3U8,
	"transcode_download_ts":            otherTranscodeDownloadTS,
	"transcode_download_all":           otherTranscodeDownloadAll,

	// image hosting (图床)
	"oss_mkdir":            otherOssMkdir,
	"oss_list":             otherOssList,
	"oss_detail":           otherOssDetail,
	"oss_move":             otherOssMove,
	"oss_delete":           otherOssDelete,
	"oss_copy_from_disk":   otherOssCopyFromDisk,
	"oss_copy_process":     otherOssCopyProcess,
	"oss_copy_fail_list":   otherOssCopyFailList,
	"oss_offline_download": otherOssOfflineDownload,
	"oss_offline_process":  otherOssOfflineProcess,
}

// Other dispatches an out-of-band operation onto the open platform. Unknown
// methods report errs.NotSupport so callers can tell them apart from failures.
func (d *Open123) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	method := strings.ToLower(strings.TrimSpace(args.Method))
	handler, ok := otherHandlers[method]
	if !ok {
		return nil, errs.NotSupport
	}
	if err := d.ensureToken(ctx); err != nil {
		return nil, err
	}
	res, err := handler(d, ctx, args)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", method, err)
	}
	return res, nil
}

// OtherMethods lists, in alphabetical order, every method Other understands,
// so callers can discover the surface.
func OtherMethods() []string {
	methods := make([]string, 0, len(otherHandlers))
	for name := range otherHandlers {
		methods = append(methods, name)
	}
	sort.Strings(methods)
	return methods
}

// decodeOtherArgs round-trips the loosely typed args.Data through JSON into a
// per-method request struct.
func decodeOtherArgs(data interface{}, target interface{}) error {
	if data == nil {
		return nil
	}
	raw, err := utils.Json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode arguments: %w", err)
	}
	if err := utils.Json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode arguments: %w", err)
	}
	return nil
}

// resolveFileID prefers an explicitly passed id and falls back to the object
// the request was addressed to.
func resolveFileID(args model.OtherArgs, explicit int64) (int64, error) {
	if explicit != 0 {
		return explicit, nil
	}
	if args.Obj != nil {
		return parseFileID(args.Obj.GetID())
	}
	return 0, errors.New("file_id is required")
}

// resolveFileIDs falls back to the addressed object when no list was given.
func resolveFileIDs(args model.OtherArgs, explicit []int64) ([]int64, error) {
	if len(explicit) > 0 {
		return explicit, nil
	}
	if args.Obj == nil {
		return nil, errors.New("file_ids is required")
	}
	id, err := parseFileID(args.Obj.GetID())
	if err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

// Shared request and response shapes.
type (
	fileIDRequest struct {
		FileID int64 `json:"file_id"`
	}
	fileIDsRequest struct {
		FileIDs []int64 `json:"file_ids"`
	}
	// okResult answers the operations the platform reports nothing about.
	okResult struct {
		Success bool `json:"success"`
	}
	taskIDResult struct {
		TaskID int64 `json:"task_id"`
	}
)

// ---------------------------------------------------------------- user

func otherUserInfo(d *Open123, ctx context.Context, _ model.OtherArgs) (interface{}, error) {
	return d.client.User.Info(ctx)
}

// ---------------------------------------------------------------- files

func otherDetail(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	return d.client.File.Detail(ctx, fileID)
}

func otherInfos(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDsRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	return d.client.File.Infos(ctx, fileIDs)
}

func otherTrash(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDsRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	if err := d.client.File.Trash(ctx, fileIDs); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherRecover(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDsRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	abnormal, err := d.client.File.Recover(ctx, fileIDs)
	if err != nil {
		return nil, err
	}
	return struct {
		AbnormalFileIDs []int64 `json:"abnormal_file_ids"`
	}{AbnormalFileIDs: abnormal}, nil
}

func otherRecoverTo(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileIDs      []int64 `json:"file_ids"`
		ParentFileID int64   `json:"parent_file_id"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	if err := d.client.File.RecoverTo(ctx, fileIDs, req.ParentFileID); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherBatchRename(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	// JSON object keys are strings, so the id keys are parsed here.
	var req struct {
		Renames map[string]string `json:"renames"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if len(req.Renames) == 0 {
		return nil, errors.New("renames is required")
	}
	renames := make(map[int64]string, len(req.Renames))
	for id, name := range req.Renames {
		fileID, err := parseFileID(id)
		if err != nil {
			return nil, err
		}
		renames[fileID] = name
	}
	return d.client.File.BatchRename(ctx, renames)
}

func otherListV1(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		ParentFileID   int64  `json:"parent_file_id"`
		Page           int    `json:"page"`
		Limit          int    `json:"limit"`
		OrderBy        string `json:"order_by"`
		OrderDirection string `json:"order_direction"`
		Trashed        bool   `json:"trashed"`
		SearchData     string `json:"search_data"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.File.ListV1(ctx, &pan123.FileListV1Request{
		ParentFileID:   req.ParentFileID,
		Page:           req.Page,
		Limit:          req.Limit,
		OrderBy:        req.OrderBy,
		OrderDirection: req.OrderDirection,
		Trashed:        req.Trashed,
		SearchData:     req.SearchData,
	})
}

func otherSafeboxID(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if req.Password == "" {
		return nil, errors.New("password is required")
	}
	fileID, err := d.client.File.SafeboxID(ctx, req.Password)
	if err != nil {
		return nil, err
	}
	return struct {
		FileID int64 `json:"file_id"`
	}{FileID: fileID}, nil
}

// ---------------------------------------------------------------- share

// shareCreateArgs carries the settings shared by free and paid share links.
type shareCreateArgs struct {
	ShareName          string  `json:"share_name"`
	FileIDs            []int64 `json:"file_ids"`
	ShareExpire        int     `json:"share_expire"`
	SharePwd           string  `json:"share_pwd"`
	PayAmount          int     `json:"pay_amount"`
	IsReward           int     `json:"is_reward"`
	ResourceDesc       string  `json:"resource_desc"`
	TrafficSwitch      int     `json:"traffic_switch"`
	TrafficLimitSwitch int     `json:"traffic_limit_switch"`
	TrafficLimit       int64   `json:"traffic_limit"`
}

type shareListArgs struct {
	Limit       int   `json:"limit"`
	LastShareID int64 `json:"last_share_id"`
}

type shareUpdateArgs struct {
	ShareIDs           []int64 `json:"share_ids"`
	TrafficSwitch      int     `json:"traffic_switch"`
	TrafficLimitSwitch int     `json:"traffic_limit_switch"`
	TrafficLimit       int64   `json:"traffic_limit"`
}

func (a shareUpdateArgs) toRequest() (*pan123.ShareUpdateRequest, error) {
	if len(a.ShareIDs) == 0 {
		return nil, errors.New("share_ids is required")
	}
	return &pan123.ShareUpdateRequest{
		ShareIDs:           a.ShareIDs,
		TrafficSwitch:      pan123.TrafficSwitch(a.TrafficSwitch),
		TrafficLimitSwitch: a.TrafficLimitSwitch,
		TrafficLimit:       a.TrafficLimit,
	}, nil
}

func otherShareCreate(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req shareCreateArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	return d.client.Share.Create(ctx, &pan123.ShareCreateRequest{
		ShareName:          req.ShareName,
		ShareExpire:        req.ShareExpire,
		FileIDs:            fileIDs,
		SharePwd:           req.SharePwd,
		TrafficSwitch:      pan123.TrafficSwitch(req.TrafficSwitch),
		TrafficLimitSwitch: req.TrafficLimitSwitch,
		TrafficLimit:       req.TrafficLimit,
	})
}

func otherShareList(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req shareListArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Share.List(ctx, req.Limit, req.LastShareID)
}

func otherShareUpdate(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req shareUpdateArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	update, err := req.toRequest()
	if err != nil {
		return nil, err
	}
	if err := d.client.Share.Update(ctx, update); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherShareCreatePaid(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req shareCreateArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	return d.client.Share.CreatePaid(ctx, &pan123.PaidShareCreateRequest{
		ShareName:          req.ShareName,
		FileIDs:            fileIDs,
		PayAmount:          req.PayAmount,
		IsReward:           req.IsReward,
		ResourceDesc:       req.ResourceDesc,
		TrafficSwitch:      pan123.TrafficSwitch(req.TrafficSwitch),
		TrafficLimitSwitch: req.TrafficLimitSwitch,
		TrafficLimit:       req.TrafficLimit,
	})
}

func otherShareListPaid(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req shareListArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Share.ListPaid(ctx, req.Limit, req.LastShareID)
}

func otherShareUpdatePaid(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req shareUpdateArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	update, err := req.toRequest()
	if err != nil {
		return nil, err
	}
	if err := d.client.Share.UpdatePaid(ctx, update); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

// ---------------------------------------------------------------- direct link

func otherDirectLinkEnable(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	folderID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	filename, err := d.client.Link.Enable(ctx, folderID)
	if err != nil {
		return nil, err
	}
	return struct {
		Filename string `json:"filename"`
	}{Filename: filename}, nil
}

func otherDirectLinkDisable(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	folderID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	filename, err := d.client.Link.Disable(ctx, folderID)
	if err != nil {
		return nil, err
	}
	return struct {
		Filename string `json:"filename"`
	}{Filename: filename}, nil
}

func otherDirectLinkURL(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	rawURL, err := d.client.Link.URL(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return struct {
		URL string `json:"url"`
	}{URL: rawURL}, nil
}

func otherDirectLinkRefreshCache(d *Open123, ctx context.Context, _ model.OtherArgs) (interface{}, error) {
	if err := d.client.Link.RefreshCache(ctx); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherDirectLinkTraffic(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		PageNum   int    `json:"page_num"`
		PageSize  int    `json:"page_size"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Link.TrafficLog(ctx, req.PageNum, req.PageSize, req.StartTime, req.EndTime)
}

func otherDirectLinkOfflineLogs(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		PageNum   int    `json:"page_num"`
		PageSize  int    `json:"page_size"`
		StartHour string `json:"start_hour"`
		EndHour   string `json:"end_hour"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Link.OfflineLogs(ctx, req.PageNum, req.PageSize, req.StartHour, req.EndHour)
}

func otherDirectLinkIPBlacklist(d *Open123, ctx context.Context, _ model.OtherArgs) (interface{}, error) {
	ips, status, err := d.client.Link.IPBlacklist(ctx)
	if err != nil {
		return nil, err
	}
	return struct {
		IPList []string `json:"ip_list"`
		Status int      `json:"status"`
	}{IPList: ips, Status: int(status)}, nil
}

func otherDirectLinkIPBlacklistUpdate(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	// The platform replaces the whole list, so an empty list clears it and is
	// a legitimate request.
	var req struct {
		IPList []string `json:"ip_list"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if err := d.client.Link.UpdateIPBlacklist(ctx, req.IPList); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherDirectLinkIPBlacklistSwitch(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		Status int `json:"status"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if req.Status != int(pan123.IPBlacklistEnabled) && req.Status != int(pan123.IPBlacklistDisabled) {
		return nil, fmt.Errorf("status must be %d (enabled) or %d (disabled)",
			pan123.IPBlacklistEnabled, pan123.IPBlacklistDisabled)
	}
	done, err := d.client.Link.SwitchIPBlacklist(ctx, pan123.IPBlacklistStatus(req.Status))
	if err != nil {
		return nil, err
	}
	return okResult{Success: done}, nil
}

// ---------------------------------------------------------------- offline download

// offlineDownloadArgs is shared by the cloud disk and the image hosting task.
type offlineDownloadArgs struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	DirID       int64  `json:"dir_id"`
	CallBackURL string `json:"call_back_url"`
}

func (a offlineDownloadArgs) toRequest(args model.OtherArgs) (*pan123.OfflineDownloadRequest, error) {
	if a.URL == "" {
		return nil, errors.New("url is required")
	}
	dirID := a.DirID
	if dirID == 0 && args.Obj != nil && args.Obj.IsDir() {
		parsed, err := parseFileID(args.Obj.GetID())
		if err != nil {
			return nil, err
		}
		dirID = parsed
	}
	return &pan123.OfflineDownloadRequest{
		URL:         a.URL,
		FileName:    a.FileName,
		DirID:       dirID,
		CallBackURL: a.CallBackURL,
	}, nil
}

type taskIDArgs struct {
	TaskID int64 `json:"task_id"`
}

func otherOfflineDownload(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req offlineDownloadArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	download, err := req.toRequest(args)
	if err != nil {
		return nil, err
	}
	taskID, err := d.client.Offline.Download(ctx, download)
	if err != nil {
		return nil, err
	}
	return taskIDResult{TaskID: taskID}, nil
}

func otherOfflineProcess(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req taskIDArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Offline.Process(ctx, req.TaskID)
}

// ---------------------------------------------------------------- transcoding

// transcodeListArgs mirrors pan123.FileListRequest for the two listing calls
// the transcoding service offers.
type transcodeListArgs struct {
	ParentFileID int64  `json:"parent_file_id"`
	Limit        int    `json:"limit"`
	SearchData   string `json:"search_data"`
	SearchMode   int    `json:"search_mode"`
	LastFileID   int64  `json:"last_file_id"`
}

func (a transcodeListArgs) toRequest() *pan123.FileListRequest {
	return &pan123.FileListRequest{
		ParentFileID: a.ParentFileID,
		Limit:        a.Limit,
		SearchData:   a.SearchData,
		SearchMode:   a.SearchMode,
		LastFileID:   a.LastFileID,
	}
}

func otherTranscodeFolderInfo(d *Open123, ctx context.Context, _ model.OtherArgs) (interface{}, error) {
	fileID, err := d.client.Transcode.FolderInfo(ctx)
	if err != nil {
		return nil, err
	}
	return struct {
		FileID int64 `json:"file_id"`
	}{FileID: fileID}, nil
}

func otherTranscodeCloudVideoFiles(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req transcodeListArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Transcode.CloudVideoFiles(ctx, req.toRequest())
}

func otherTranscodeSpaceFiles(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req transcodeListArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Transcode.SpaceFiles(ctx, req.toRequest())
}

func otherTranscodeUploadFromCloudDisk(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDsRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	if err := d.client.Transcode.UploadFromCloudDisk(ctx, fileIDs); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherTranscodeResolutions(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	return d.client.Transcode.Resolutions(ctx, fileID)
}

func otherTranscodeVideo(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileID      int64  `json:"file_id"`
		CodecName   string `json:"codec_name"`
		VideoTime   int64  `json:"video_time"`
		Resolutions string `json:"resolutions"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	if req.Resolutions == "" {
		return nil, errors.New("resolutions is required")
	}
	err = d.client.Transcode.Transcode(ctx, &pan123.TranscodeRequest{
		FileID:      fileID,
		CodecName:   req.CodecName,
		VideoTime:   req.VideoTime,
		Resolutions: req.Resolutions,
	})
	if err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherTranscodeRecords(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	return d.client.Transcode.Records(ctx, fileID)
}

func otherTranscodeResults(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	return d.client.Transcode.Results(ctx, fileID)
}

func otherTranscodeList(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	return d.client.Transcode.List(ctx, fileID)
}

func otherTranscodeDelete(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileID int64 `json:"file_id"`
		// Mode 1 deletes the source only, 2 also deletes the transcoded files.
		Mode int `json:"mode"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	if req.Mode == 0 {
		req.Mode = int(pan123.DeleteOriginal)
	}
	if req.Mode != int(pan123.DeleteOriginal) && req.Mode != int(pan123.DeleteOriginalAndTranscoded) {
		return nil, fmt.Errorf("mode must be %d (original) or %d (original and transcoded)",
			pan123.DeleteOriginal, pan123.DeleteOriginalAndTranscoded)
	}
	if err := d.client.Transcode.Delete(ctx, fileID, pan123.DeleteMode(req.Mode)); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherTranscodeDownloadOriginal(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req fileIDRequest
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	return d.client.Transcode.DownloadOriginal(ctx, fileID)
}

func otherTranscodeDownloadM3U8(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileID     int64  `json:"file_id"`
		Resolution string `json:"resolution"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	if req.Resolution == "" {
		return nil, errors.New("resolution is required")
	}
	return d.client.Transcode.DownloadM3U8(ctx, fileID, req.Resolution)
}

func otherTranscodeDownloadTS(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileID     int64  `json:"file_id"`
		Resolution string `json:"resolution"`
		TsName     string `json:"ts_name"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	if req.Resolution == "" || req.TsName == "" {
		return nil, errors.New("resolution and ts_name are required")
	}
	return d.client.Transcode.DownloadTS(ctx, fileID, req.Resolution, req.TsName)
}

func otherTranscodeDownloadAll(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileID  int64  `json:"file_id"`
		ZipName string `json:"zip_name"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileID, err := resolveFileID(args, req.FileID)
	if err != nil {
		return nil, err
	}
	if req.ZipName == "" {
		return nil, errors.New("zip_name is required")
	}
	return d.client.Transcode.DownloadAll(ctx, fileID, req.ZipName)
}

// ---------------------------------------------------------------- image hosting

// Image hosting file IDs are strings and live in their own namespace, so they
// are never taken from the addressed object.

func otherOssMkdir(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		ParentID string `json:"parent_id"`
		Name     string `json:"name"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, errors.New("name is required")
	}
	dirID, err := d.client.Oss.Mkdir(ctx, req.ParentID, req.Name)
	if err != nil {
		return nil, err
	}
	return struct {
		DirID string `json:"dir_id"`
	}{DirID: dirID}, nil
}

func otherOssList(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		ParentFileID string `json:"parent_file_id"`
		Limit        int    `json:"limit"`
		StartTime    int64  `json:"start_time"`
		EndTime      int64  `json:"end_time"`
		LastFileID   string `json:"last_file_id"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Oss.List(ctx, &pan123.OssListRequest{
		ParentFileID: req.ParentFileID,
		Limit:        req.Limit,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		LastFileID:   req.LastFileID,
	})
}

func otherOssDetail(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileID string `json:"file_id"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if req.FileID == "" {
		return nil, errors.New("file_id is required")
	}
	return d.client.Oss.Detail(ctx, req.FileID)
}

func otherOssMove(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileIDs        []string `json:"file_ids"`
		ToParentFileID string   `json:"to_parent_file_id"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if len(req.FileIDs) == 0 {
		return nil, errors.New("file_ids is required")
	}
	if req.ToParentFileID == "" {
		return nil, errors.New("to_parent_file_id is required")
	}
	if err := d.client.Oss.Move(ctx, req.FileIDs, req.ToParentFileID); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherOssDelete(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileIDs []string `json:"file_ids"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if len(req.FileIDs) == 0 {
		return nil, errors.New("file_ids is required")
	}
	if err := d.client.Oss.Delete(ctx, req.FileIDs); err != nil {
		return nil, err
	}
	return okResult{Success: true}, nil
}

func otherOssCopyFromDisk(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		FileIDs        []int64 `json:"file_ids"`
		ToParentFileID string  `json:"to_parent_file_id"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	fileIDs, err := resolveFileIDs(args, req.FileIDs)
	if err != nil {
		return nil, err
	}
	taskID, err := d.client.Oss.CopyFromDisk(ctx, fileIDs, req.ToParentFileID)
	if err != nil {
		return nil, err
	}
	return struct {
		TaskID string `json:"task_id"`
	}{TaskID: taskID}, nil
}

func otherOssCopyProcess(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if req.TaskID == "" {
		return nil, errors.New("task_id is required")
	}
	status, failMsg, err := d.client.Oss.CopyProcess(ctx, req.TaskID)
	if err != nil {
		return nil, err
	}
	return struct {
		Status  int    `json:"status"`
		FailMsg string `json:"fail_msg"`
	}{Status: int(status), FailMsg: failMsg}, nil
}

func otherOssCopyFailList(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req struct {
		TaskID string `json:"task_id"`
		Page   int    `json:"page"`
		Limit  int    `json:"limit"`
	}
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	if req.TaskID == "" {
		return nil, errors.New("task_id is required")
	}
	return d.client.Oss.CopyFailList(ctx, req.TaskID, req.Page, req.Limit)
}

func otherOssOfflineDownload(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req offlineDownloadArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	// the image hosting namespace has its own directory ids, so the addressed
	// object is not a usable fallback here
	if req.URL == "" {
		return nil, errors.New("url is required")
	}
	taskID, err := d.client.Oss.OfflineDownload(ctx, &pan123.OfflineDownloadRequest{
		URL:         req.URL,
		FileName:    req.FileName,
		DirID:       req.DirID,
		CallBackURL: req.CallBackURL,
	})
	if err != nil {
		return nil, err
	}
	return taskIDResult{TaskID: taskID}, nil
}

func otherOssOfflineProcess(d *Open123, ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var req taskIDArgs
	if err := decodeOtherArgs(args.Data, &req); err != nil {
		return nil, err
	}
	return d.client.Oss.OfflineProcess(ctx, req.TaskID)
}
