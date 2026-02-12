import { useState, useEffect, useCallback } from 'react';
import { 
  RefreshCw, 
  Settings, 
  History, 
  Image as ImageIcon,
  CheckCircle2,
  AlertCircle,
  ExternalLink,
  Trash2,
  Eraser
} from 'lucide-react';
import { toast } from 'sonner';

import { 
  GetConfig, 
  SaveConfig, 
  FetchToday, 
  ApplyWallpaper, 
  ListHistory, 
  ApplyHistory, 
  DeleteHistory, 
  ClearHistory, 
  GetImageDataURL,
  OpenDataDir,
  OpenLogsDir,
  CleanupByRetainDays,
  GetWallpaperSupport
} from '../wailsjs/go/app/App';
import { store } from '../wailsjs/go/models';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from '@/components/ui/drawer';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ScrollArea } from '@/components/ui/scroll-area';

function App() {
  const [config, setConfig] = useState<store.Config | null>(null);
  const [currentImage, setCurrentImage] = useState<store.HistoryItem | null>(null);
  const [currentImageDataURL, setCurrentImageDataURL] = useState<string>('');
  const [history, setHistory] = useState<store.HistoryItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [wallpaperSupport, setWallpaperSupport] = useState<any>(true);

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
      setHistory(items.reverse()); // Newest first
    } catch (err) {
      console.error('Failed to load history:', err);
    }
  }, []);

  // Get today's wallpaper
  const fetchToday = useCallback(async () => {
    setLoading(true);
    try {
      const w = window.screen.width * window.devicePixelRatio;
      const h = window.screen.height * window.devicePixelRatio;
      const dpr = window.devicePixelRatio;

      const result = await FetchToday(Math.round(w), Math.round(h), dpr);
      if (result.success) {
        setCurrentImage(result.item);
        loadHistory();
        toast.success('已获取今日壁纸: ' + result.item.title);
      } else {
        toast.error('获取今日壁纸失败: ' + result.error);
      }
    } catch (err) {
      toast.error('请求失败: ' + err);
    } finally {
      setLoading(false);
    }
  }, [loadHistory]);

  // Initial load
  useEffect(() => {
    loadConfig();
    loadHistory();
    fetchToday();
    GetWallpaperSupport().then(setWallpaperSupport);
  }, []);

  // Update background image when currentImage changes
  useEffect(() => {
    if (currentImage) {
      const path = currentImage.image_path;
      GetImageDataURL(path).then(setCurrentImageDataURL).catch(console.error);
    }
  }, [currentImage]);

  const handleApplyWallpaper = async () => {
    if (!currentImage) return;
    setLoading(true);
    try {
      await ApplyWallpaper(config?.overlay_metadata || false);
      toast.success('壁纸设置成功');
    } catch (err) {
      toast.error('设置壁纸失败: ' + err);
    } finally {
      setLoading(false);
    }
  };

  const handleSaveConfig = async (newCfg: store.Config) => {
    try {
      await SaveConfig(newCfg);
      setConfig(newCfg);
      toast.success('配置已保存');
    } catch (err) {
      toast.error('保存配置失败: ' + err);
    }
  };

  const handleApplyHistory = async (item: store.HistoryItem) => {
    setLoading(true);
    try {
      await ApplyHistory(item.key, config?.overlay_metadata || false);
      setCurrentImage(item);
      toast.success('已切换并设为壁纸');
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
      const count = await CleanupByRetainDays();
      loadHistory();
      toast.info(`清理完成，删除了 ${count} 条过旧记录`);
    } catch (err) {
      toast.error('清理失败: ' + err);
    }
  };

  return (
    <div className="relative h-full w-full bg-slate-950 overflow-hidden font-sans">
      {/* Background Image */}
      <div 
        className="absolute inset-0 bg-cover bg-center transition-all duration-700 ease-in-out"
        style={{ 
          backgroundImage: currentImageDataURL ? `url(${currentImageDataURL})` : 'none',
          filter: 'brightness(0.7)'
        }}
      />
      <div className="absolute inset-0 bg-gradient-to-b from-black/30 via-transparent to-black/60 pointer-events-none" />

      {/* Top Left: Status & Quick Toggles */}
      <div className="absolute top-6 left-6 flex flex-col gap-4 z-10">
        <div className="flex items-center space-x-2 bg-black/40 backdrop-blur-md p-2 px-3 rounded-full border border-white/10">
          <Switch 
            id="auto-apply" 
            checked={config?.auto_apply || false}
            onCheckedChange={(val) => config && handleSaveConfig({ ...config, auto_apply: val })}
          />
          <Label htmlFor="auto-apply" className="text-xs font-light text-white cursor-pointer select-none">自动设置</Label>
        </div>
        <div className="flex items-center space-x-2 bg-black/40 backdrop-blur-md p-2 px-3 rounded-full border border-white/10">
          <Switch 
            id="overlay" 
            checked={config?.overlay_metadata || false}
            onCheckedChange={(val) => config && handleSaveConfig({ ...config, overlay_metadata: val })}
          />
          <Label htmlFor="overlay" className="text-xs font-light text-white cursor-pointer select-none">叠加水印</Label>
        </div>
      </div>

      {/* Top Right: Actions */}
      <div className="absolute top-6 right-6 flex gap-3 z-10">
        <Drawer>
          <DrawerTrigger asChild>
            <Button variant="ghost" size="icon" className="bg-black/40 backdrop-blur-md border border-white/10 rounded-full hover:bg-black/60 text-white">
              <History className="h-5 w-5" />
            </Button>
          </DrawerTrigger>
          <DrawerContent className="max-h-[85vh]">
            <DrawerHeader className="border-b">
              <div className="flex items-center justify-between">
                <DrawerTitle>壁纸历史</DrawerTitle>
                <Button variant="ghost" size="sm" onClick={handleClearHistory} className="text-destructive hover:text-destructive">
                  <Eraser className="h-4 w-4 mr-2" />
                  清空全部
                </Button>
              </div>
            </DrawerHeader>
            <ScrollArea className="h-full p-4">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 pb-8">
                {history.map((item) => (
                  <HistoryItemCard 
                    key={item.key} 
                    item={item} 
                    onApply={() => handleApplyHistory(item)}
                    onDelete={() => handleDeleteHistory(item.key)}
                  />
                ))}
                {history.length === 0 && (
                  <div className="col-span-full py-20 text-center text-muted-foreground">
                    暂无历史记录
                  </div>
                )}
              </div>
            </ScrollArea>
          </DrawerContent>
        </Drawer>

        <Dialog>
          <DialogTrigger asChild>
            <Button variant="ghost" size="icon" className="bg-black/40 backdrop-blur-md border border-white/10 rounded-full hover:bg-black/60 text-white">
              <Settings className="h-5 w-5" />
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[500px]">
            <DialogHeader>
              <DialogTitle>设置</DialogTitle>
            </DialogHeader>
            <div className="grid gap-6 py-4">
              <div className="grid gap-2">
                <Label>API 地址</Label>
                <Input 
                  value={config?.api_meta_url || ''} 
                  onChange={(e) => config && setConfig({ ...config, api_meta_url: e.target.value })}
                />
              </div>
              
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>调度模式</Label>
                  <Select 
                    value={config?.schedule_mode} 
                    onValueChange={(val) => config && setConfig({ ...config, schedule_mode: val })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="off">禁用</SelectItem>
                      <SelectItem value="daily">每日固定时间</SelectItem>
                      <SelectItem value="interval">固定间隔</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {config?.schedule_mode === 'daily' && (
                  <div className="grid gap-2">
                    <Label>触发时间</Label>
                    <Input 
                      type="time" 
                      value={config?.daily_time} 
                      onChange={(e) => setConfig({ ...config, daily_time: e.target.value })}
                    />
                  </div>
                )}
                {config?.schedule_mode === 'interval' && (
                  <div className="grid gap-2">
                    <Label>间隔 (分钟, ≥15)</Label>
                    <Input 
                      type="number" 
                      min={15}
                      value={config?.interval_minutes} 
                      onChange={(e) => setConfig({ ...config, interval_minutes: parseInt(e.target.value) })}
                    />
                  </div>
                )}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>保留天数 (0=永久)</Label>
                  <Input 
                    type="number" 
                    min={0}
                    value={config?.retain_days} 
                    onChange={(e) => config && setConfig({ ...config, retain_days: parseInt(e.target.value) })}
                  />
                </div>
                <div className="flex flex-col justify-end">
                  <Button variant="outline" size="sm" onClick={handleCleanup}>
                    立即执行清理
                  </Button>
                </div>
              </div>

              <div className="flex flex-col gap-4 border-t pt-4">
                 <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>优先宽高比匹配</Label>
                      <p className="text-[0.7rem] text-muted-foreground">优先选择比例接近屏幕的分辨率</p>
                    </div>
                    <Switch 
                      checked={config?.prefer_aspect_match}
                      onCheckedChange={(val) => config && setConfig({ ...config, prefer_aspect_match: val })}
                    />
                 </div>
                 <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>强制 4K (UHD)</Label>
                      <p className="text-[0.7rem] text-muted-foreground">忽略比例，存在 UHD 则必选</p>
                    </div>
                    <Switch 
                      checked={config?.force_uhd}
                      onCheckedChange={(val) => config && setConfig({ ...config, force_uhd: val })}
                    />
                 </div>
              </div>

              <div className="flex gap-2 pt-2">
                <Button variant="secondary" className="flex-1" onClick={() => OpenDataDir()}>数据目录</Button>
                <Button variant="secondary" className="flex-1" onClick={() => OpenLogsDir()}>日志目录</Button>
              </div>
            </div>
            <DialogFooter>
              <Button onClick={() => config && handleSaveConfig(config)}>保存配置</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* Center: Wallpaper Info */}
      <div className="absolute bottom-12 left-12 right-12 flex flex-col md:flex-row items-end justify-between gap-6 pointer-events-none">
        <div className="flex flex-col gap-2 max-w-2xl pointer-events-auto">
          {currentImage && (
            <>
              <h1 className="text-3xl md:text-5xl font-bold text-white drop-shadow-lg leading-tight">
                {currentImage.title}
              </h1>
              <p className="text-sm md:text-base text-white/80 drop-shadow-md">
                {currentImage.copyright}
              </p>
              <div className="flex items-center gap-3 mt-2">
                <span className="text-xs bg-black/40 backdrop-blur-sm px-2 py-0.5 rounded border border-white/10 text-white/60">
                  {currentImage.date}
                </span>
                <span className="text-xs bg-black/40 backdrop-blur-sm px-2 py-0.5 rounded border border-white/10 text-white/60">
                  {currentImage.chosen_variant}
                </span>
              </div>
            </>
          )}
        </div>

        <div className="flex gap-3 pointer-events-auto">
          <Button 
            size="lg" 
            variant="secondary"
            className="rounded-full px-8 shadow-xl hover:scale-105 transition-transform"
            onClick={fetchToday}
            disabled={loading}
          >
            <RefreshCw className={cn("h-5 w-5 mr-2", loading && "animate-spin")} />
            刷新
          </Button>
          <Button 
            size="lg" 
            className="rounded-full px-8 shadow-xl bg-white text-black hover:bg-white/90 hover:scale-105 transition-transform"
            onClick={handleApplyWallpaper}
            disabled={loading || !currentImage}
          >
            <ImageIcon className="h-5 w-5 mr-2" />
            设为壁纸
          </Button>
        </div>
      </div>

      {/* Linux Support Notice */}
      {typeof wallpaperSupport === 'string' && (
        <div className="absolute bottom-6 left-1/2 -translate-x-1/2 bg-yellow-500/90 text-black text-[10px] px-3 py-1 rounded-full font-medium z-50">
          注意: {wallpaperSupport}
        </div>
      )}
    </div>
  );
}

function HistoryItemCard({ item, onApply, onDelete }: { 
  item: store.HistoryItem, 
  onApply: () => void,
  onDelete: () => void
}) {
  const [thumb, setThumb] = useState<string>('');

  useEffect(() => {
    GetImageDataURL(item.image_path).then(setThumb).catch(console.error);
  }, [item.image_path]);

  return (
    <div className="group relative overflow-hidden rounded-xl border bg-card text-card-foreground shadow-sm hover:shadow-md transition-all">
      <div className="aspect-video w-full overflow-hidden bg-muted">
        {thumb ? (
          <img src={thumb} alt={item.title} className="h-full w-full object-cover transition-transform group-hover:scale-105" />
        ) : (
          <div className="h-full w-full flex items-center justify-center">
            <ImageIcon className="h-8 w-8 text-muted-foreground/50" />
          </div>
        )}
      </div>
      <div className="p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1">
            <h4 className="text-sm font-semibold leading-none truncate max-w-[150px]">{item.title}</h4>
            <p className="text-xs text-muted-foreground">{item.date}</p>
          </div>
          <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onApply}>
              <CheckCircle2 className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive" onClick={onDelete}>
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
