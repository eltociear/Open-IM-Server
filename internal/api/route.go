package api

import (
	"OpenIM/internal/api/auth"
	"OpenIM/internal/api/conversation"
	"OpenIM/internal/api/friend"
	"OpenIM/internal/api/group"
	"OpenIM/internal/api/manage"
	"OpenIM/internal/api/msg"
	"OpenIM/internal/api/third"
	"OpenIM/internal/api/user"
	"OpenIM/pkg/common/config"
	"OpenIM/pkg/common/log"
	"OpenIM/pkg/common/middleware"
	"OpenIM/pkg/common/prome"
	"OpenIM/pkg/common/tokenverify"
	"github.com/gin-gonic/gin"
	"io"
	"os"
)

func NewGinRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	f, _ := os.Create("../logs/api.log")
	gin.DefaultWriter = io.MultiWriter(f)
	//	gin.SetMode(gin.DebugMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.GinParseOperationID)
	log.Info("load config: ", config.Config)
	if config.Config.Prometheus.Enable {
		prome.NewApiRequestCounter()
		prome.NewApiRequestFailedCounter()
		prome.NewApiRequestSuccessCounter()
		r.Use(prome.PrometheusMiddleware)
		r.GET("/metrics", prome.PrometheusHandler())
	}
	userRouterGroup := r.Group("/user")
	{
		userRouterGroup.POST("/update_user_info", user.UpdateUserInfo) //1
		userRouterGroup.POST("/set_global_msg_recv_opt", user.SetGlobalRecvMessageOpt)
		userRouterGroup.POST("/get_users_info", user.GetUsersPublicInfo)            //1
		userRouterGroup.POST("/get_self_user_info", user.GetSelfUserInfo)           //1
		userRouterGroup.POST("/get_users_online_status", user.GetUsersOnlineStatus) //1
		userRouterGroup.POST("/get_users_info_from_cache", user.GetUsersInfoFromCache)
		userRouterGroup.POST("/get_user_friend_from_cache", user.GetFriendIDListFromCache)
		userRouterGroup.POST("/get_black_list_from_cache", user.GetBlackIDListFromCache)
		userRouterGroup.POST("/get_all_users_uid", manage.GetAllUsersUid) //1
		userRouterGroup.POST("/account_check", manage.AccountCheck)       //1
		//	userRouterGroup.POST("/get_users_online_status", manage.GetUsersOnlineStatus) //1
		userRouterGroup.POST("/get_users", user.GetUsers)
	}
	////friend routing group
	friendRouterGroup := r.Group("/friend")
	{
		//	friendRouterGroup.POST("/get_friends_info", friend.GetFriendsInfo)
		friendRouterGroup.POST("/add_friend", friend.AddFriend)                        //1
		friendRouterGroup.POST("/delete_friend", friend.DeleteFriend)                  //1
		friendRouterGroup.POST("/get_friend_apply_list", friend.GetFriendApplyList)    //1
		friendRouterGroup.POST("/get_self_friend_apply_list", friend.GetSelfApplyList) //1
		friendRouterGroup.POST("/get_friend_list", friend.GetFriendList)               //1
		friendRouterGroup.POST("/add_friend_response", friend.AddFriendResponse)       //1
		friendRouterGroup.POST("/set_friend_remark", friend.SetFriendRemark)           //1

		friendRouterGroup.POST("/add_black", friend.AddBlack)           //1
		friendRouterGroup.POST("/get_black_list", friend.GetBlacklist)  //1
		friendRouterGroup.POST("/remove_black", friend.RemoveBlacklist) //1

		friendRouterGroup.POST("/import_friend", friend.ImportFriend) //1
		friendRouterGroup.POST("/is_friend", friend.IsFriend)         //1
	}
	//group related routing group
	groupRouterGroup := r.Group("/group")
	groupRouterGroup.Use(func(c *gin.Context) {
		userID, err := tokenverify.ParseUserIDFromToken(c.GetHeader("token"), c.GetString("operationID"))
		if err != nil {
			c.String(400, err.Error())
			c.Abort()
			return
		}
		c.Set("opUserID", userID)
		c.Next()
	})
	{
		groupRouterGroup.POST("/create_group", group.NewCreateGroup)                                //1
		groupRouterGroup.POST("/set_group_info", group.NewSetGroupInfo)                             //1
		groupRouterGroup.POST("/join_group", group.JoinGroup)                                       //1
		groupRouterGroup.POST("/quit_group", group.QuitGroup)                                       //1
		groupRouterGroup.POST("/group_application_response", group.ApplicationGroupResponse)        //1
		groupRouterGroup.POST("/transfer_group", group.TransferGroupOwner)                          //1
		groupRouterGroup.POST("/get_recv_group_applicationList", group.GetRecvGroupApplicationList) //1
		groupRouterGroup.POST("/get_user_req_group_applicationList", group.GetUserReqGroupApplicationList)
		groupRouterGroup.POST("/get_groups_info", group.GetGroupsInfo) //1
		groupRouterGroup.POST("/kick_group", group.KickGroupMember)    //1
		//	groupRouterGroup.POST("/get_group_member_list", group.FindGroupMemberAll)        //no use
		groupRouterGroup.POST("/get_group_all_member_list", group.GetGroupAllMemberList) //1
		groupRouterGroup.POST("/get_group_members_info", group.GetGroupMembersInfo)      //1
		groupRouterGroup.POST("/invite_user_to_group", group.InviteUserToGroup)          //1
		groupRouterGroup.POST("/get_joined_group_list", group.GetJoinedGroupList)
		groupRouterGroup.POST("/dismiss_group", group.DismissGroup) //
		groupRouterGroup.POST("/mute_group_member", group.MuteGroupMember)
		groupRouterGroup.POST("/cancel_mute_group_member", group.CancelMuteGroupMember) //MuteGroup
		groupRouterGroup.POST("/mute_group", group.MuteGroup)
		groupRouterGroup.POST("/cancel_mute_group", group.CancelMuteGroup)
		groupRouterGroup.POST("/set_group_member_nickname", group.SetGroupMemberNickname)
		groupRouterGroup.POST("/set_group_member_info", group.SetGroupMemberInfo)
		groupRouterGroup.POST("/get_group_abstract_info", group.GetGroupAbstractInfo)
		//groupRouterGroup.POST("/get_group_all_member_list_by_split", group.GetGroupAllMemberListBySplit)
	}
	superGroupRouterGroup := r.Group("/super_group")
	{
		superGroupRouterGroup.POST("/get_joined_group_list", group.GetJoinedSuperGroupList)
		superGroupRouterGroup.POST("/get_groups_info", group.GetSuperGroupsInfo)
	}
	////certificate
	authRouterGroup := r.Group("/auth")
	{
		authRouterGroup.POST("/user_register", apiAuth.UserRegister) //1
		authRouterGroup.POST("/user_token", apiAuth.UserToken)       //1
		authRouterGroup.POST("/parse_token", apiAuth.ParseToken)     //1
		authRouterGroup.POST("/force_logout", apiAuth.ForceLogout)   //1
	}
	////Third service
	thirdGroup := r.Group("/third")
	{
		thirdGroup.POST("/tencent_cloud_storage_credential", third.TencentCloudStorageCredential)
		thirdGroup.POST("/ali_oss_credential", third.AliOSSCredential)
		thirdGroup.POST("/minio_storage_credential", third.MinioStorageCredential)
		thirdGroup.POST("/minio_upload", third.MinioUploadFile)
		thirdGroup.POST("/upload_update_app", third.UploadUpdateApp)
		thirdGroup.POST("/get_download_url", third.GetDownloadURL)
		thirdGroup.POST("/get_rtc_invitation_info", third.GetRTCInvitationInfo)
		thirdGroup.POST("/get_rtc_invitation_start_app", third.GetRTCInvitationInfoStartApp)
		thirdGroup.POST("/fcm_update_token", third.FcmUpdateToken)
		thirdGroup.POST("/aws_storage_credential", third.AwsStorageCredential)
		thirdGroup.POST("/set_app_badge", third.SetAppBadge)
	}
	////Message
	chatGroup := r.Group("/msg")
	{
		chatGroup.POST("/newest_seq", msg.GetSeq)
		chatGroup.POST("/send_msg", msg.SendMsg)
		chatGroup.POST("/pull_msg_by_seq", msg.PullMsgBySeqList)
		chatGroup.POST("/del_msg", msg.DelMsg)
		chatGroup.POST("/del_super_group_msg", msg.DelSuperGroupMsg)
		chatGroup.POST("/clear_msg", msg.ClearMsg)
		chatGroup.POST("/manage_send_msg", manage.ManagementSendMsg)
		chatGroup.POST("/batch_send_msg", manage.ManagementBatchSendMsg)
		chatGroup.POST("/check_msg_is_send_success", manage.CheckMsgIsSendSuccess)
		chatGroup.POST("/set_msg_min_seq", msg.SetMsgMinSeq)

		chatGroup.POST("/set_message_reaction_extensions", msg.SetMessageReactionExtensions)
		chatGroup.POST("/get_message_list_reaction_extensions", msg.GetMessageListReactionExtensions)
		chatGroup.POST("/add_message_reaction_extensions", msg.AddMessageReactionExtensions)
		chatGroup.POST("/delete_message_reaction_extensions", msg.DeleteMessageReactionExtensions)
	}
	////Conversation
	conversationGroup := r.Group("/conversation")
	{ //1
		conversationGroup.POST("/get_all_conversations", conversation.GetAllConversations)
		conversationGroup.POST("/get_conversation", conversation.GetConversation)
		conversationGroup.POST("/get_conversations", conversation.GetConversations)
		conversationGroup.POST("/set_conversation", conversation.SetConversation)
		conversationGroup.POST("/batch_set_conversation", conversation.BatchSetConversations)
		conversationGroup.POST("/set_recv_msg_opt", conversation.SetRecvMsgOpt)
		conversationGroup.POST("/modify_conversation_field", conversation.ModifyConversationField)
	}
	return r
}