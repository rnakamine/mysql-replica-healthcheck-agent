package main

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInnerHandler(t *testing.T) {
	tests := []struct {
		name      string
		config    ReplicaConfig
		mockSetup func(mock sqlmock.Sqlmock)
		want      map[string]interface{}
		wantErr   bool
	}{
		{
			name: "slave status available with no errors",
			config: ReplicaConfig{
				FailSlaveNotRunning:    false,
				MaxSecondsBehindMaster: 10,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"Seconds_Behind_Master"}).
					AddRow("0")
				mock.ExpectQuery(regexp.QuoteMeta("SHOW SLAVE STATUS")).
					WillReturnRows(rows)
			},
			want: map[string]interface{}{
				"Seconds_Behind_Master": int64(0),
			},
			wantErr: false,
		},
		{
			name: "slave is not running with no slave status",
			config: ReplicaConfig{
				FailSlaveNotRunning:    true,
				MaxSecondsBehindMaster: 10,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"Seconds_Behind_Master"})
				mock.ExpectQuery(regexp.QuoteMeta("SHOW SLAVE STATUS")).
					WillReturnRows(rows)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "slave is up and running with replication lag too high",
			config: ReplicaConfig{
				FailSlaveNotRunning:    true,
				MaxSecondsBehindMaster: 10,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"Seconds_Behind_Master"}).
					AddRow("30")
				mock.ExpectQuery(regexp.QuoteMeta("SHOW SLAVE STATUS")).
					WillReturnRows(rows)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			got, err := innerHandler(&tt.config, db)
			if (err != nil) != tt.wantErr {
				t.Errorf("innerHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("innerHandler() = %v, want %v", got, tt.want)
			}
		})
	}
}
