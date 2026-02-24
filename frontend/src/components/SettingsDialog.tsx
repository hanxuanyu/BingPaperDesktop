import { useState, useEffect } from 'react';
import { Settings, Info, Github } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { store, app } from '../../wailsjs/go/models';
import { 
  OpenDataDir, 
  OpenLogsDir, 
  Quit, 
  GetVersionInfo, 
  BrowserOpenURL,
  GetBaseDir,
  SelectDirectory,
  SetBaseDir 
} from '../../wailsjs/go/app/App';

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  config: store.Config | null;
  platform: string;
  onSaveConfig: (newCfg: store.Config, closeDialog: boolean) => Promise<void>;
  onCleanup: () => Promise<void>;
  onCleanupLogs: () => Promise<void>;
  onReset: () => void;
}

export function SettingsDialog({
  open,
  onOpenChange,
  config: initialConfig,
  platform,
  onSaveConfig,
  onCleanup,
  onCleanupLogs,
  onReset
}: SettingsDialogProps) {
  const [localConfig, setLocalConfig] = useState<store.Config | null>(null);
  const [versionInfo, setVersionInfo] = useState<app.VersionInfo | null>(null);
  const [currentBaseDir, setCurrentBaseDir] = useState<string>('');

  useEffect(() => {
    if (open && initialConfig) {
      setLocalConfig({ ...initialConfig });
      GetVersionInfo().then(setVersionInfo);
      GetBaseDir().then(setCurrentBaseDir);
    }
  }, [open, initialConfig]);

  const handleSelectBaseDir = async () => {
    try {
      const selected = await SelectDirectory();
      if (selected && selected !== currentBaseDir) {
        await SetBaseDir(selected);
        setCurrentBaseDir(selected);
        onReset();
      }
    } catch (err) {
      console.error('Failed to select directory:', err);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" className="bg-black/40 backdrop-blur-md border border-white/10 rounded-full hover:bg-black/60 text-white">
          <Settings className="h-5 w-5" />
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[600px] max-h-[90vh] overflow-hidden p-0 gap-0">
        {!localConfig ? (
          <div className="p-12 flex items-center justify-center">
             <span className="text-muted-foreground animate-pulse">加载配置中...</span>
          </div>
        ) : (
          <>
            <DialogHeader className="p-6 pb-2 shrink-0">
              <DialogTitle className="text-xl">应用设置</DialogTitle>
            </DialogHeader>
            <div className="flex-1 min-h-0 overflow-y-auto">
                <div className="space-y-8 py-4 px-6 pb-12">
                  {/* API Section */}
                  <section className="space-y-4">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">接口设置</h3>
                    <div className="grid gap-6 rounded-lg border p-4 bg-muted/20">
                      <div className="flex items-start justify-between gap-4">
                        <div className="space-y-1">
                          <Label className="text-base">API 接口类型</Label>
                          <p className="text-xs text-muted-foreground">
                            {localConfig.api_type === 'bing' 
                              ? '官方接口由微软提供，支持获取当日最新的壁纸。' 
                              : 'BingPaper 接口支持随机获取和获取当日壁纸，并支持多种宽高比自适应。'}
                          </p>
                        </div>
                        <Select 
                          value={localConfig.api_type} 
                          onValueChange={(val) => {
                            setLocalConfig({ ...localConfig, api_type: val });
                          }}
                        >
                          <SelectTrigger className="w-[180px]">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="bing">必应官方</SelectItem>
                            <SelectItem value="custom">BingPaper</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="space-y-2">
                        <Label>API 地址</Label>
                        <Input 
                          className="font-mono text-xs"
                          value={localConfig.api_type === 'bing' ? (localConfig.bing_api_url || '') : (localConfig.custom_api_url || '')} 
                          onChange={(e) => {
                            const val = e.target.value;
                            if (localConfig.api_type === 'bing') {
                              setLocalConfig({ ...localConfig, bing_api_url: val });
                            } else {
                              setLocalConfig({ ...localConfig, custom_api_url: val });
                            }
                          }}
                        />
                      </div>
                    </div>
                  </section>

                  {/* Automation Section */}
                  <section className="space-y-4">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">自动调度</h3>
                    <div className="grid gap-6 rounded-lg border p-4 bg-muted/20">
                      <div className="flex items-center justify-between">
                        <div className="space-y-0.5">
                          <Label className="text-base">调度模式</Label>
                          <p className="text-xs text-muted-foreground">设置壁纸自动更新的频率</p>
                        </div>
                        <Select 
                          value={localConfig.schedule_mode} 
                          onValueChange={(val) => setLocalConfig({ ...localConfig, schedule_mode: val })}
                        >
                          <SelectTrigger className="w-[180px]">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="off">禁用</SelectItem>
                            <SelectItem value="daily">每日固定时间</SelectItem>
                            <SelectItem value="interval">固定间隔</SelectItem>
                            <SelectItem value="wakeup">休眠唤醒时触发</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      {localConfig.schedule_mode === 'daily' && (
                        <div className="flex items-center justify-between border-t pt-4">
                          <Label>触发时间</Label>
                          <Input 
                            type="time" 
                            className="w-[180px]"
                            value={localConfig.daily_time} 
                            onChange={(e) => setLocalConfig({ ...localConfig, daily_time: e.target.value })}
                          />
                        </div>
                      )}

                      {localConfig.schedule_mode === 'interval' && (
                        <div className="flex items-center justify-between border-t pt-4">
                          <Label>间隔 (分钟, ≥15)</Label>
                          <Input 
                            type="number" 
                            min={15}
                            className="w-[180px]"
                            value={localConfig.interval_minutes} 
                            onChange={(e) => setLocalConfig({ ...localConfig, interval_minutes: parseInt(e.target.value) })}
                          />
                        </div>
                      )}

                      {localConfig.schedule_mode !== 'off' && (
                        <div className="flex items-center justify-between bg-background/50 p-3 rounded-md border border-dashed mt-2">
                          <div className="space-y-0.5">
                            <Label className="text-sm">随机更换历史壁纸</Label>
                            <p className="text-[0.7rem] text-muted-foreground">每次触发时从历史记录中随机选择</p>
                          </div>
                          <Switch 
                            checked={localConfig.random_history || false}
                            onCheckedChange={(val) => setLocalConfig({ ...localConfig, random_history: val })}
                          />
                        </div>
                      )}
                    </div>
                  </section>

                  {/* Storage & Maintenance */}
                  <section className="space-y-4">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">存储与清理</h3>
                    <div className="grid gap-6 rounded-lg border p-4 bg-muted/20">
                      <div className="flex items-center justify-between">
                        <div className="space-y-0.5">
                          <Label className="text-base">保留天数</Label>
                          <p className="text-xs text-muted-foreground">0 为永久保留</p>
                        </div>
                        <div className="flex items-center gap-3">
                          <Input 
                            type="number" 
                            min={0}
                            className="w-[100px]"
                            value={localConfig.retain_days} 
                            onChange={(e) => setLocalConfig({ ...localConfig, retain_days: parseInt(e.target.value) })}
                          />
                          <Button variant="outline" size="sm" onClick={onCleanup}>
                            立即清理
                          </Button>
                        </div>
                      </div>

                      <div className="flex gap-3 border-t pt-4">
                        <Button variant="secondary" className="flex-1 h-9 text-xs" onClick={() => OpenDataDir()}>打开数据目录</Button>
                        <Button variant="secondary" className="flex-1 h-9 text-xs" onClick={() => OpenLogsDir()}>查看日志目录</Button>
                      </div>

                      <div className="space-y-2 border-t pt-4">
                        <div className="flex items-center justify-between">
                          <Label className="text-sm font-medium">基准保存路径 (baseDir)</Label>
                          <Button variant="outline" size="sm" className="h-7 text-[0.7rem]" onClick={handleSelectBaseDir}>
                            更改路径
                          </Button>
                        </div>
                        <div className="flex items-center gap-2 bg-muted/30 p-2 rounded border text-[0.7rem] font-mono break-all">
                          <span className="truncate flex-1">{currentBaseDir}</span>
                        </div>
                        <p className="text-[0.65rem] text-muted-foreground italic">
                          更改此路径将切换配置文件和数据的保存位置，旧数据不会自动迁移。
                        </p>
                      </div>
                    </div>
                  </section>

                  {/* Other settings */}
                  {(localConfig.api_type === 'custom' || true) && (
                    <section className="space-y-4">
                      <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">其他选项</h3>
                      <div className="grid gap-4 rounded-lg border p-4 bg-muted/20">
                        {localConfig.api_type === 'custom' && (
                          <div className="flex items-center justify-between">
                            <div className="space-y-0.5">
                              <Label className="text-base">强制 4K (UHD)</Label>
                              <p className="text-xs text-muted-foreground">忽略屏幕比例，优先选择 UHD 资源</p>
                            </div>
                            <Switch 
                              checked={localConfig.force_uhd || false}
                              onCheckedChange={(val) => setLocalConfig({ ...localConfig, force_uhd: val })}
                            />
                          </div>
                        )}

                        <div className="flex items-center justify-between">
                          <div className="space-y-0.5">
                            <Label className="text-base">开机自启动</Label>
                            <p className="text-xs text-muted-foreground">在系统启动后自动运行应用</p>
                          </div>
                          <Switch 
                            checked={localConfig.auto_start || false}
                            onCheckedChange={(val) => setLocalConfig({ ...localConfig, auto_start: val })}
                          />
                        </div>

                        {platform === 'darwin' && (
                          <div className="flex items-center justify-between">
                            <div className="space-y-0.5">
                              <Label className="text-base">隐藏 Dock 图标</Label>
                              <p className="text-xs text-muted-foreground">仅在状态栏显示，不在 Dock 栏显示图标</p>
                            </div>
                            <Switch 
                              checked={localConfig.hide_dock_icon || false}
                              onCheckedChange={(val) => setLocalConfig({ ...localConfig, hide_dock_icon: val })}
                            />
                          </div>
                        )}

                        <div className="border-t pt-4 space-y-4">
                          <h4 className="text-sm font-medium">日志管理</h4>
                          <div className="grid grid-cols-2 gap-4">
                            <div className="space-y-2">
                              <Label className="text-xs">日志保留天数</Label>
                              <div className="flex items-center gap-2">
                                <Input 
                                  type="number" 
                                  className="h-8"
                                  value={localConfig.log_retain_days}
                                  onChange={(e) => setLocalConfig({ ...localConfig, log_retain_days: parseInt(e.target.value) || 0 })}
                                />
                                <span className="text-xs text-muted-foreground shrink-0">天</span>
                              </div>
                            </div>
                            <div className="space-y-2">
                              <Label className="text-xs">单文件最大尺寸</Label>
                              <div className="flex items-center gap-2">
                                <Input 
                                  type="number" 
                                  className="h-8"
                                  value={localConfig.log_max_size}
                                  onChange={(e) => setLocalConfig({ ...localConfig, log_max_size: parseInt(e.target.value) || 0 })}
                                />
                                <span className="text-xs text-muted-foreground shrink-0">MB</span>
                              </div>
                            </div>
                          </div>
                          <Button 
                            variant="outline" 
                            size="sm" 
                            className="w-full text-xs h-8"
                            onClick={onCleanupLogs}
                          >
                            手动清理并压缩旧日志
                          </Button>
                        </div>
                        
                        <div className="pt-2">
                          <Button 
                            variant="outline" 
                            className="w-full border-destructive/50 text-destructive hover:bg-destructive/10 hover:border-destructive" 
                            onClick={onReset}
                          >
                            重置所有应用数据与配置
                          </Button>
                        </div>
                      </div>
                    </section>
                  )}

                  {/* Version Information */}
                  <section className="space-y-4">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">关于</h3>
                    <div className="rounded-lg border p-4 bg-muted/20 space-y-3">
                      <div className="flex items-center gap-3">
                        <div className="p-2 bg-primary/10 rounded-full">
                          <Info className="h-4 w-4 text-primary" />
                        </div>
                        <div>
                          <p className="text-sm font-medium">BingPaperDesktop</p>
                          <p className="text-xs text-muted-foreground">让你的桌面每天都有新发现</p>
                        </div>
                      </div>
                      
                      <div className="grid grid-cols-2 gap-x-4 gap-y-2 pt-2 text-xs border-t mt-2">
                        <div className="text-muted-foreground">当前版本</div>
                        <div className="font-mono text-right">{versionInfo?.version || '加载中...'}</div>
                        <div className="text-muted-foreground">构建哈希</div>
                        <div className="font-mono text-right">{versionInfo?.commit_hash || '加载中...'}</div>
                        <div className="text-muted-foreground">构建时间</div>
                        <div className="text-right text-muted-foreground/80">{versionInfo?.build_time || '加载中...'}</div>
                      </div>

                      <div className="pt-2 border-t flex justify-center">
                        <Button 
                          variant="link" 
                          size="sm" 
                          className="text-xs h-7 text-muted-foreground"
                          onClick={() => BrowserOpenURL('https://github.com/hanxuanyu/BingPaperDesktop')}
                        >
                          <Github className="h-3 w-3 mr-1" />
                          GitHub 仓库
                        </Button>
                      </div>
                    </div>
                  </section>
                </div>
            </div>
            <DialogFooter className="p-6 border-t flex flex-row justify-between items-center bg-muted/10 shrink-0">
              <Button variant="ghost" size="sm" className="text-muted-foreground hover:text-destructive transition-colors" onClick={() => Quit()}>
                彻底退出应用
              </Button>
              <div className="flex gap-2">
                <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
                <Button onClick={() => onSaveConfig(localConfig, true)}>保存配置</Button>
              </div>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
