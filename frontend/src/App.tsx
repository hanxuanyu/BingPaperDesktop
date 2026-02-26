import { useState, useEffect, useCallback, useRef } from 'react';
import { toast } from 'sonner';

import { 
  TooltipProvider
} from '@/components/ui/tooltip';
import { EventsOn, Environment } from '../wailsjs/runtime';
import { 
  GetConfig, 
  SaveConfig, 
  FetchToday, 
  ListHistory, 
  ApplyHistory, 
  ApplyHistoryToMonitor,
  GetMonitors,
  DeleteHistory, 
  ClearHistory, 
  GetImageURL,
  GetThumbnailURL,
  CleanupByRetainDays,
  CleanupLogs,
  GetWallpaperSupport,
  SubmitWatermark,
  ResetApplication,
  ResetSettings,
  GetCurrentItem,
  GetMonitorWallpapers
} from '../wailsjs/go/app/App';
import { store } from '../wailsjs/go/models';
import { renderWatermark } from './lib/watermark';

import { QuickToggles } from './components/QuickToggles';
import { HistoryDrawer } from './components/HistoryDrawer';
import { SettingsDialog } from './components/SettingsDialog';
import { WallpaperInfo } from './components/WallpaperInfo';
import { ResetDialog } from './components/ResetDialog';
import { MonitorSwitcher } from './components/MonitorSwitcher';

function App() {
  const [config, setConfig] = useState<store.Config | null>(null);
  const [currentImage, setCurrentImage] = useState<store.HistoryItem | null>(null);
  const [currentImageDataURL, setCurrentImageDataURL] = useState<string>('');
  const [prevImageDataURL, setPrevImageDataURL] = useState<string>('');
  const [isImgLoading, setIsImgLoading] = useState(false);
  const [history, setHistory] = useState<store.HistoryItem[]>([]);
  const [monitors, setMonitors] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [wallpaperSupport, setWallpaperSupport] = useState<any>(true);
  const [platform, setPlatform] = useState<string>('');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isResetDialogOpen, setIsResetDialogOpen] = useState(false);
  const isManualFetching = useRef(false);
  const configRef = useRef<store.Config | null>(null);

  // Sync config to ref for event listeners
  useEffect(() => {
    configRef.current = config;
  }, [config]);

  // Load configuration
  const loadConfig = useCallback(async () => {
    try {
      const cfg = await GetConfig();
      setConfig(cfg);
    } catch (err) {
      toast.error('加载配置失败: ' + err);
    }
  }, []);

  // Load history
  const loadHistory = useCallback(async () => {
    try {
      const items = await ListHistory();
      setHistory(items); // Use the order from backend
    } catch (err) {
      console.error('Failed to load history:', err);
    }
  }, []);

  // Load monitors
  const loadMonitors = useCallback(async () => {
    try {
      const m = await GetMonitors();
      setMonitors(m || []);
    } catch (err) {
      // 静默失败，避免托盘/多显示器不可用时刷屏
    }
  }, []);

  // Get today's wallpaper
  const fetchToday = useCallback(async (silent = false) => {
    setLoading(true);
    if (!silent) isManualFetching.current = true;
    try {
      // 注意：window.screen.width/height 返回的是 CSS 逻辑像素
      // Go 端的 FetchToday 会自行将逻辑像素乘以 dpr 得到物理像素
      // 因此这里不能提前乘以 dpr，否则会导致分辨率被双倍放大（DPR² 问题）
      const result = await FetchToday(window.screen.width, window.screen.height, window.devicePixelRatio);
      if (result.success) {
        setCurrentImage(store.HistoryItem.createFrom(result.item));
        loadHistory();
        if (!silent) {
          const prefix = config?.random_history ? '当前为随机壁纸: ' : '已获取今日壁纸: ';
          toast.success(prefix + result.item.title);
        }
      } else {
        toast.error('获取今日壁纸失败: ' + result.error);
      }
    } catch (err) {
      toast.error('请求失败: ' + err);
    } finally {
      setLoading(false);
      isManualFetching.current = false;
    }
  }, [loadHistory, config?.random_history]);

  // Initial load
  useEffect(() => {
    loadConfig();
    loadHistory();
    loadMonitors();
    fetchToday(true);
    GetWallpaperSupport().then(setWallpaperSupport);
    Environment().then(env => setPlatform(env.platform));

    // Sync current image listener
    const unregisterSync = EventsOn('current-image-changed', (item: store.HistoryItem) => {
      if (item) {
        setCurrentImage((prev) => {
          if (prev && prev.key !== item.key && !isManualFetching.current) {
            const prefix = configRef.current?.random_history ? '当前为随机壁纸: ' : '壁纸已同步: ';
            toast.info(prefix + item.title);
          }
          if (!prev || prev.key !== item.key) {
            loadHistory();
          }
          return store.HistoryItem.createFrom(item);
        });
      }
    });

    const handleFocus = () => {
      // 当窗口重新获得焦点时，清除所有积压的通知
      toast.dismiss();
    };
    window.addEventListener('focus', handleFocus);

    const unregisterShow = EventsOn('prepare-show-window', () => {
      toast.dismiss();
      // 在显示窗口前主动拉取最新状态，确保主界面和桌面一致
      GetCurrentItem().then(result => {
        if (result && result.success && result.item) {
          setCurrentImage(store.HistoryItem.createFrom(result.item));
        }
      }).catch(console.error);
    });

    // Watermark/Overlay rendering listener
    const unregister = EventsOn('render-watermark', async (data: any) => {
      try {
        const base64Data = await renderWatermark(data);
        if (!base64Data) {
           throw new Error("Render returned empty data");
        }
        await SubmitWatermark(base64Data);
      } catch (err) {
        console.error('Render error:', err);
        toast.error('图片叠加渲染失败: ' + err);
        await SubmitWatermark(""); // Fallback
      }
    });

    return () => {
      unregister();
      unregisterSync();
      unregisterShow();
      window.removeEventListener('focus', handleFocus);
    };
  }, []);

  // Update background image when currentImage changes
  useEffect(() => {
    if (currentImage) {
      // 主界面始终显示原图
      const path = currentImage.image_path;
      
      GetImageURL(path).then(async (url) => {
        if (url === currentImageDataURL) return;

        // 预加载图片确保切换时不闪烁
        const img = new Image();
        img.src = url;
        setIsImgLoading(true);
        try {
          await img.decode();
        } catch (e) {
          console.warn("Image decode failed", e);
        }

        setPrevImageDataURL(currentImageDataURL);
        setCurrentImageDataURL(url);
        setIsImgLoading(false);
      }).catch(console.error);
    } else {
      setPrevImageDataURL(currentImageDataURL);
      setCurrentImageDataURL('');
    }
  }, [currentImage]);

  const handleApplyWallpaper = async (monitorId: number = -1) => {
    if (!currentImage) return;
    setLoading(true);
    try {
      const w = Math.round(window.screen.width * window.devicePixelRatio);
      const h = Math.round(window.screen.height * window.devicePixelRatio);
      await ApplyHistoryToMonitor(currentImage.key, monitorId, w, h);
      setCurrentImage(store.HistoryItem.createFrom(currentImage)); // Trigger refresh
      toast.success(monitorId === -1 ? '所有屏幕壁纸设置成功' : `屏幕 ${monitorId + 1} 壁纸设置成功`);
    } catch (err) {
      toast.error('设置壁纸失败: ' + err);
    } finally {
      setLoading(false);
    }
  };

  const handleSaveConfig = async (newCfg: store.Config, closeDialog = false) => {
    try {
      await SaveConfig(newCfg);
      
      const holidaySwitchedOn = !config?.enable_holiday && newCfg.enable_holiday;
      const holidayUrlChanged = config?.enable_holiday && newCfg.enable_holiday && config?.holiday_api_url !== newCfg.holiday_api_url;
      
      setConfig(newCfg);
      toast.success('配置已保存');

      if (holidaySwitchedOn) {
        toast.info('节假日显示已开启，正在后台下载数据...');
      } else if (holidayUrlChanged) {
        toast.info('节假日数据源已更新，正在重新下载数据...');
      }

      if (currentImage) {
        // 配置更新后立即应用以触发重新渲染预览
        handleApplyHistory(currentImage, -1);
      }
      if (closeDialog) {
        setIsSettingsOpen(false);
      }
    } catch (err) {
      toast.error('保存配置失败: ' + err);
    }
  };

  const handleApplyHistory = async (item: store.HistoryItem, monitorId: number = -1) => {
    setLoading(true);
    try {
      const w = Math.round(window.screen.width * window.devicePixelRatio);
      const h = Math.round(window.screen.height * window.devicePixelRatio);
      await ApplyHistoryToMonitor(item.key, monitorId, w, h);
      setCurrentImage(store.HistoryItem.createFrom(item));
      toast.success(monitorId === -1 ? '已切换并设为壁纸' : `已切换并设为屏幕 ${monitorId + 1} 壁纸`);
    } catch (err) {
      toast.error('设置壁纸失败: ' + err);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteHistory = async (key: string) => {
    try {
      await DeleteHistory(key);
      loadHistory();
      toast.success('已删除');
    } catch (err) {
      toast.error('删除失败: ' + err);
    }
  };

  const handleClearHistory = async () => {
    if (!confirm('确定要清空所有历史记录吗？这也会删除本地图片文件。')) return;
    try {
      await ClearHistory();
      loadHistory();
      setCurrentImage(null);
      setCurrentImageDataURL('');
      toast.success('已清空历史记录');
    } catch (err) {
      toast.error('清空失败: ' + err);
    }
  };

  const handleCleanup = async () => {
    try {
      const days = config?.retain_days || 0;
      if (days <= 0) {
        toast.warning('请先设置保留天数（需大于 0）');
        return;
      }
      const count = await CleanupByRetainDays(days);
      loadHistory();
      toast.success(`清理完成，已删除 ${count} 条历史记录`);
    } catch (err) {
      toast.error('清理失败: ' + err);
    }
  };

  const onResetConfirm = async (onlySettings: boolean) => {
    setIsResetDialogOpen(false);
    const tid = toast.loading(onlySettings ? '正在重置应用配置...' : '正在重置应用并清理数据...');
    try {
      if (onlySettings) {
        await ResetSettings();
      } else {
        await ResetApplication();
      }
      toast.success(onlySettings ? '应用配置已恢复默认' : '应用数据已清空，配置已恢复默认', { id: tid });
      setIsSettingsOpen(false);
      await loadConfig();
      if (!onlySettings) {
        await loadHistory();
        setCurrentImage(null);
        setCurrentImageDataURL('');
        fetchToday(true);
      }
    } catch (err) {
      toast.error('重置失败: ' + err, { id: tid });
    }
  };

  const handleCleanupLogs = async () => {
    try {
      await CleanupLogs();
      toast.success('日志清理已触发');
    } catch (err) {
      toast.error('日志清理失败: ' + err);
    }
  };

  return (
    <TooltipProvider>
      <div className="relative h-full w-full bg-slate-950 overflow-hidden font-sans">
        {/* Background Image Layers */}
        {prevImageDataURL && (
          <div 
            className="absolute inset-0 bg-cover bg-center"
            style={{ 
              backgroundImage: `url(${prevImageDataURL})`,
              filter: 'brightness(0.7)'
            }}
          />
        )}
        <div 
          key={currentImageDataURL}
          className="absolute inset-0 bg-cover bg-center transition-opacity duration-700 ease-in-out animate-in fade-in"
          style={{ 
            backgroundImage: currentImageDataURL ? `url(${currentImageDataURL})` : 'none',
            filter: 'brightness(0.7)'
          }}
        />
        <div className="absolute inset-0 bg-gradient-to-b from-black/30 via-transparent to-black/60 pointer-events-none" />

        {/* Top Left: Status & Quick Toggles */}
        <QuickToggles config={config} onConfigChange={(newCfg) => handleSaveConfig(newCfg)} />

        {/* Top Right: Actions */}
        <div className="absolute top-6 right-6 flex gap-3 z-10">
          <HistoryDrawer 
            history={history}
            monitors={monitors}
            onApplyHistory={handleApplyHistory}
            onDeleteHistory={handleDeleteHistory}
            onClearHistory={handleClearHistory}
          />

          <SettingsDialog 
            open={isSettingsOpen}
            onOpenChange={setIsSettingsOpen}
            config={config}
            platform={platform}
            onSaveConfig={handleSaveConfig}
            onCleanup={handleCleanup}
            onCleanupLogs={handleCleanupLogs}
            onReset={() => setIsResetDialogOpen(true)}
          />
        </div>

        {/* Center: Wallpaper Info */}
        <WallpaperInfo 
          currentImage={currentImage}
          loading={loading}
          monitors={monitors}
          onRefresh={() => fetchToday()}
          onApply={handleApplyWallpaper}
        />

        {/* Floating Monitor Switcher */}
        <MonitorSwitcher 
          currentImage={currentImage}
          onSelect={(item) => setCurrentImage(store.HistoryItem.createFrom(item))}
        />

        {/* Linux Support Notice */}
        {typeof wallpaperSupport === 'string' && (
          <div className="absolute bottom-6 left-1/2 -translate-x-1/2 bg-yellow-500/90 text-black text-[10px] px-3 py-1 rounded-full font-medium z-50">
            注意: {wallpaperSupport}
          </div>
        )}

        {/* Reset Confirmation Dialog */}
        <ResetDialog 
          open={isResetDialogOpen}
          onOpenChange={setIsResetDialogOpen}
          onConfirm={onResetConfirm}
        />
      </div>
    </TooltipProvider>
  );
}

export default App;
