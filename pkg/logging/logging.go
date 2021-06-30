/*
Copyright 2021 Metacontroller authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"go.uber.org/zap/zapcore"
	controllerruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

var (
	// Logger is global json log format logr
	Logger logr.Logger
)

func InitLogging(opts *controllerruntimezap.Options) {
	if opts.Development {
		opts.EncoderConfigOptions = append(opts.EncoderConfigOptions, func(encoderConfig *zapcore.EncoderConfig) {
			encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		})
	}
	Logger = controllerruntimezap.New(controllerruntimezap.UseFlagOptions(opts))
	klog.SetLogger(Logger)
	controllerruntimelog.SetLogger(Logger)
}
