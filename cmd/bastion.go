package cmd

import (
	"crypto/tls"
	"fmt"
	"github.com/fatih/color"
	log "github.com/mborsuk/jwalterweatherman"
	"github.com/opsee/basic/service"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"regexp"
	"time"
)

const uuidFormat = `^[a-z0-9]{8}-[a-z0-9]{4}-[1-5][a-z0-9]{3}-[a-z0-9]{4}-[a-z0-9]{12}$`
const emailFormat = `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`

const tcpTimeout = time.Duration(1) * time.Second

type bastionServices struct {
	Vape     service.VapeClient
	Spanx    service.SpanxClient
	Keelhaul service.KeelhaulClient
}

func NewBastionServices(vape service.VapeClient, spanx service.SpanxClient, keelhaul service.KeelhaulClient) *bastionServices {
	return &bastionServices{
		Vape:     vape,
		Spanx:    spanx,
		Keelhaul: keelhaul,
	}
}

var svcs *bastionServices

// bastionCmd represents the bastion command
var bastionCmd = &cobra.Command{
	Use: "bastion",
}

var bastionListCmd = &cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {

		if len(args) < 1 {
			return NewUserError("missing argument")
		}

		email, uuid, err := parseUserID(args[0])
		if err != nil {
			return err
		}

		userResp, err := svcs.Vape.GetUser(context.Background(), &service.GetUserRequest{
			Email:      email,
			CustomerId: uuid,
		})
		if err != nil {
			return err
		}
		_ = userResp

		keelResp, err := svcs.Keelhaul.ListBastionStates(context.Background(), &service.ListBastionStatesRequest{
			CustomerIds: []string{userResp.User.CustomerId},
		})
		if err != nil {
			return err
		}
		yellow := color.New(color.FgYellow).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		blue := color.New(color.FgBlue).SprintFunc()
		for _, b := range keelResp.BastionStates {
			lastSeenDur := time.Since(time.Unix(b.LastSeen.Seconds, 0))
			fmt.Printf("%s %s %s\n", yellow(b.Id), blue(b.Status), red(roundDuration(lastSeenDur, time.Second)))
		}

		return nil
	},
}

func roundDuration(d, r time.Duration) time.Duration {
	if r <= 0 {
		return d
	}
	neg := d < 0
	if neg {
		d = -d
	}
	if m := d % r; m+m < r {
		d = d - m
	} else {
		d = d + r - m
	}
	if neg {
		return -d
	}
	return d
}

func parseUserID(id string) (email string, uuid string, err error) {
	emailExp := regexp.MustCompile(emailFormat)
	uuidExp := regexp.MustCompile(uuidFormat)

	if emailExp.MatchString(id) {
		return id, "", nil
	}

	if uuidExp.MatchString(id) {
		return "", id, nil
	}

	return "", "", NewUserError("no email or UUID found in string")
}

func init() {
	log.SetLogFlag(log.SFILE)

	BoopCmd.AddCommand(bastionCmd)
	bastionCmd.AddCommand(bastionListCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// bastionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// bastionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// TODO get endpoints from yaml config w/Viper
	conn, err := grpc.Dial("vape.in.opsee.com:443",
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithTimeout(tcpTimeout),
		grpc.WithBlock())
	if err != nil {
		log.ERROR.Fatal(err)
	}
	vape := service.NewVapeClient(conn)

	conn, err = grpc.Dial("spanx.in.opsee.com:8443",
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithTimeout(tcpTimeout))
	if err != nil {
		panic(err)
	}
	spanx := service.NewSpanxClient(conn)

	conn, err = grpc.Dial("keelhaul.in.opsee.com:443",
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithTimeout(tcpTimeout))
	if err != nil {
		panic(err)
	}
	keelhaul := service.NewKeelhaulClient(conn)

	svcs = NewBastionServices(vape, spanx, keelhaul)

}