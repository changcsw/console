package main

import (
	"fmt"
	"os"
	"time"

	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 临时集成测试令牌签发器（测试专家用，验证后删除）。
func main() {
	secret := os.Getenv("ADMIN_JWT_SECRET")
	iss, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret: secret, Issuer: "admin-api", AccessTTL: 60 * time.Minute, RefreshTTL: 336 * time.Hour,
	})
	if err != nil {
		panic(err)
	}
	perms := []string{"channel.read", "channel.write"}
	if os.Getenv("NO_WRITE") == "1" {
		perms = []string{"channel.read"}
	}
	ac := domainauth.NewAuthContext(1, "it", "IT", []string{"channel_admin"}, perms, common.Environment("public"))
	pair, err := iss.IssuePair(ac)
	if err != nil {
		panic(err)
	}
	fmt.Print(pair.AccessToken)
}
