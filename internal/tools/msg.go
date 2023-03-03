package tools

import (
	"OpenIM/pkg/common/config"
	"OpenIM/pkg/common/constant"
	"OpenIM/pkg/common/db/cache"
	"OpenIM/pkg/common/db/controller"
	"OpenIM/pkg/common/db/relation"
	"OpenIM/pkg/common/db/unrelation"
	"OpenIM/pkg/common/log"
	"OpenIM/pkg/common/tracelog"
	"OpenIM/pkg/utils"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"math"
)

type MsgTool struct {
	msgDatabase   controller.MsgDatabase
	userDatabase  controller.UserDatabase
	groupDatabase controller.GroupDatabase
}

func NewMsgTool(msgDatabase controller.MsgDatabase, userDatabase controller.UserDatabase, groupDatabase controller.GroupDatabase) *MsgTool {
	return &MsgTool{
		msgDatabase:   msgDatabase,
		userDatabase:  userDatabase,
		groupDatabase: groupDatabase,
	}
}

func InitMsgTool() (*MsgTool, error) {
	rdb, err := cache.NewRedis()
	if err != nil {
		return nil, err
	}
	mongo, err := unrelation.NewMongo()
	if err != nil {
		return nil, err
	}
	db, err := relation.NewGormDB()
	if err != nil {
		return nil, err
	}
	msgDatabase := controller.InitMsgDatabase(rdb, mongo.GetDatabase())
	userDatabase := controller.NewUserDatabase(relation.NewUserGorm(db))
	groupDatabase := controller.InitGroupDatabase(db, rdb, mongo.GetDatabase())
	msgTool := NewMsgTool(msgDatabase, userDatabase, groupDatabase)
	return msgTool, nil
}

func (c *MsgTool) getCronTaskOperationID() string {
	return cronTaskOperationID + utils.OperationIDGenerator()
}

func (c *MsgTool) AllUserClearMsgAndFixSeq() {
	operationID := c.getCronTaskOperationID()
	ctx := context.Background()
	tracelog.SetOperationID(ctx, operationID)
	log.NewInfo(operationID, "============================ start del cron task ============================")
	var err error
	userIDList, err := c.userDatabase.GetAllUserID(ctx)
	if err == nil {
		c.ClearUsersMsg(ctx, userIDList)
	} else {
		log.NewError(operationID, utils.GetSelfFuncName(), err.Error())
	}
	// working group msg clear
	superGroupIDList, err := c.groupDatabase.GetGroupIDsByGroupType(ctx, constant.WorkingGroup)
	if err == nil {
		c.ClearSuperGroupMsg(ctx, superGroupIDList)
	} else {
		log.NewError(operationID, utils.GetSelfFuncName(), err.Error())
	}
	log.NewInfo(operationID, "============================ start del cron finished ============================")
}

func (c *MsgTool) ClearUsersMsg(ctx context.Context, userIDs []string) {
	for _, userID := range userIDs {
		if err := c.msgDatabase.DeleteUserMsgsAndSetMinSeq(ctx, userID, int64(config.Config.Mongo.DBRetainChatRecords*24*60*60)); err != nil {
			log.NewError(tracelog.GetOperationID(ctx), utils.GetSelfFuncName(), err.Error(), userID)
		}
		maxSeqCache, maxSeqMongo, err := c.GetAndFixUserSeqs(ctx, userID)
		if err != nil {
			continue
		}
		c.CheckMaxSeqWithMongo(ctx, userID, maxSeqCache, maxSeqMongo, constant.WriteDiffusion)
	}
}

func (c *MsgTool) ClearSuperGroupMsg(ctx context.Context, superGroupIDs []string) {
	for _, groupID := range superGroupIDs {
		userIDs, err := c.groupDatabase.FindGroupMemberUserID(ctx, groupID)
		if err != nil {
			log.NewError(tracelog.GetOperationID(ctx), utils.GetSelfFuncName(), "FindGroupMemberUserID", err.Error(), groupID)
			continue
		}
		if err := c.msgDatabase.DeleteUserSuperGroupMsgsAndSetMinSeq(ctx, groupID, userIDs, int64(config.Config.Mongo.DBRetainChatRecords*24*60*60)); err != nil {
			log.NewError(tracelog.GetOperationID(ctx), utils.GetSelfFuncName(), err.Error(), "DeleteUserSuperGroupMsgsAndSetMinSeq failed", groupID, userIDs, config.Config.Mongo.DBRetainChatRecords)
		}
		if err := c.fixGroupSeq(ctx, groupID, userIDs); err != nil {
			log.NewError(tracelog.GetOperationID(ctx), utils.GetSelfFuncName(), err.Error(), groupID, userIDs)
		}
	}
}

func (c *MsgTool) FixGroupSeq(ctx context.Context, groupID string) error {
	userIDs, err := c.groupDatabase.FindGroupMemberUserID(ctx, groupID)
	if err != nil {
		return err
	}
	return c.fixGroupSeq(ctx, groupID, userIDs)
}

func (c *MsgTool) fixGroupSeq(ctx context.Context, groupID string, userIDs []string) error {
	_, maxSeqMongo, maxSeqCache, err := c.msgDatabase.GetSuperGroupMinMaxSeqInMongoAndCache(ctx, groupID)
	if err != nil {
		log.NewError(tracelog.GetOperationID(ctx), utils.GetSelfFuncName(), err.Error(), "GetUserMinMaxSeqInMongoAndCache failed", groupID)
		return err
	}
	for _, userID := range userIDs {
		if _, err := c.GetAndFixGroupUserSeq(ctx, userID, groupID, maxSeqCache); err != nil {
			log.NewError(tracelog.GetOperationID(ctx), "GetAndFixGroupUserSeq failed", groupID, userID, maxSeqCache)
			continue
		}
	}
	c.CheckMaxSeqWithMongo(ctx, groupID, maxSeqCache, maxSeqMongo, constant.WriteDiffusion)
	return nil
}

func (c *MsgTool) GetAndFixUserSeqs(ctx context.Context, userID string) (maxSeqCache, maxSeqMongo int64, err error) {
	_, maxSeqMongo, minSeqCache, maxSeqCache, err := c.msgDatabase.GetUserMinMaxSeqInMongoAndCache(ctx, userID)
	if err != nil {
		log.NewError(tracelog.GetOperationID(ctx), utils.GetSelfFuncName(), err.Error(), "GetUserMinMaxSeqInMongoAndCache failed", userID)
		return 0, 0, err
	}

	if minSeqCache > maxSeqCache {
		if err := c.msgDatabase.SetUserMinSeq(ctx, userID, maxSeqCache); err != nil {
			log.NewError(tracelog.GetOperationID(ctx), "SetUserMinSeq failed", userID, minSeqCache, maxSeqCache)
		} else {
			log.NewWarn(tracelog.GetOperationID(ctx), "SetUserMinSeq success", userID, minSeqCache, maxSeqCache)
		}
	}
	return maxSeqCache, maxSeqMongo, nil
}

func (c *MsgTool) GetAndFixGroupUserSeq(ctx context.Context, userID string, groupID string, maxSeqCache int64) (minSeqCache int64, err error) {
	minSeqCache, err = c.msgDatabase.GetGroupUserMinSeq(ctx, groupID, userID)
	if err != nil {
		log.NewError(tracelog.GetOperationID(ctx), "GetGroupUserMinSeq failed", groupID, userID)
		return 0, err
	}
	if minSeqCache > maxSeqCache {
		if err := c.msgDatabase.SetGroupUserMinSeq(ctx, groupID, userID, maxSeqCache); err != nil {
			log.NewError(tracelog.GetOperationID(ctx), "SetGroupUserMinSeq failed", userID, minSeqCache, maxSeqCache)
		} else {
			log.NewWarn(tracelog.GetOperationID(ctx), "SetGroupUserMinSeq success", userID, minSeqCache, maxSeqCache)
		}
	}
	return minSeqCache, nil
}

func (c *MsgTool) CheckMaxSeqWithMongo(ctx context.Context, sourceID string, maxSeqCache, maxSeqMongo int64, diffusionType int) {
	if math.Abs(float64(maxSeqMongo-maxSeqCache)) > 10 {
		log.NewWarn(tracelog.GetOperationID(ctx), "cache max seq and mongo max seq is diff > 10", sourceID, maxSeqCache, maxSeqMongo, diffusionType)
	}
}

func (c *MsgTool) ShowUserSeqs(ctx context.Context, userID string) {

}

func (c *MsgTool) ShowSuperGroupSeqs(ctx context.Context, groupID string) {

}

func (c *MsgTool) ShowSuperGroupUserSeqs(ctx context.Context, groupID, userID string) {

}

func (c *MsgTool) FixAllSeq(ctx context.Context) {
	userIDs, err := c.userDatabase.GetAllUserID(ctx)
	if err != nil {
		panic(err.Error())
	}
	for _, userID := range userIDs {
		userCurrentMinSeq, err := c.msgDatabase.GetUserMinSeq(ctx, userID)
		if err != nil && err != redis.Nil {
			continue
		}
		userCurrentMaxSeq, err := c.msgDatabase.GetUserMaxSeq(ctx, userID)
		if err != nil && err != redis.Nil {
			continue
		}
		if userCurrentMinSeq > userCurrentMaxSeq {
			if err = c.msgDatabase.SetUserMinSeq(ctx, userID, userCurrentMaxSeq); err != nil {
				fmt.Println("SetUserMinSeq failed", userID, userCurrentMaxSeq)
			}
			fmt.Println("fix", userID, userCurrentMaxSeq)
		}
	}
	fmt.Println("fix users seq success")
	groupIDs, err := c.groupDatabase.GetGroupIDsByGroupType(ctx, constant.WorkingGroup)
	if err != nil {
		panic(err.Error())
	}
	for _, groupID := range groupIDs {
		maxSeq, err := c.msgDatabase.GetGroupMaxSeq(ctx, groupID)
		if err != nil {
			fmt.Println("GetGroupMaxSeq failed", groupID)
			continue
		}
		userIDs, err := c.groupDatabase.FindGroupMemberUserID(ctx, groupID)
		if err != nil {
			fmt.Println("get groupID", groupID, "failed, try again later")
			continue
		}
		for _, userID := range userIDs {
			userMinSeq, err := c.msgDatabase.GetGroupUserMinSeq(ctx, groupID, userID)
			if err != nil && err != redis.Nil {
				fmt.Println("GetGroupUserMinSeq failed", groupID, userID)
				continue
			}
			if userMinSeq > maxSeq {
				if err = c.msgDatabase.SetGroupUserMinSeq(ctx, groupID, userID, maxSeq); err != nil {
					fmt.Println("SetGroupUserMinSeq failed", err.Error(), groupID, userID, maxSeq)
				}
				fmt.Println("fix", groupID, userID, maxSeq, userMinSeq)
			}
		}
	}
	fmt.Println("fix all seq finished")
}