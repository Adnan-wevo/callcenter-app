/** Mirrors internal/softphone.Bootstrap on the Go side (softphone.go). */
export interface SoftphoneExtension {
  extension: string;
  display_name: string;
  is_default: boolean;
  is_supervisor: boolean;
  queues: string[];
}

export interface SupervisorCodes {
  spy_monitor: string;
  spy_whisper: string;
  spy_barge: string;
}

export interface SoftphoneBootstrap {
  server: string;
  port: number;
  ws_path: string;
  transport: string;

  extension: string;
  display_name: string;
  password: string;
  queues: string[];

  is_supervisor: boolean;
  supervisor: SupervisorCodes;
  extensions: SoftphoneExtension[];
}
