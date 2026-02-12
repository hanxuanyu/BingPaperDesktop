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
  Tooltip, 
  TooltipContent, 
  TooltipProvider, 
  TooltipTrigger 
} from '@/components/ui/tooltip';
import { EventsOn } from '../wailsjs/runtime';
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
  GetWallpaperSupport,
  SubmitWatermark,
  Quit
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

    // Sync current image listener
    const unregisterSync = EventsOn('current-image-changed', (item: store.HistoryItem) => {
      if (item) {
        setCurrentImage(item);
        loadHistory();
        toast.info('壁纸已同步: ' + item.title);
      }
    });

    // Watermark rendering listener
    const unregister = EventsOn('render-watermark', async (data: any) => {
      try {
        const { image_path, title, date, copyright, variant } = data;
        const img = new Image();
        img.crossOrigin = "anonymous";
        img.src = image_path;
        
        await new Promise((resolve, reject) => {
          img.onload = resolve;
          img.onerror = reject;
        });

        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        if (!ctx) throw new Error('Could not get canvas context');

        canvas.width = img.width;
        canvas.height = img.height;

        // Draw original image
        ctx.drawImage(img, 0, 0);

        // Watermark style
        const paddingX = canvas.width * 0.05;
        const paddingY = canvas.height * 0.05;
        const titleFontSize = Math.max(24, Math.floor(canvas.height * 0.045));
        const copyrightFontSize = Math.max(14, Math.floor(canvas.height * 0.018));
        const tagFontSize = Math.max(12, Math.floor(canvas.height * 0.015));

        // 1. Draw bottom gradient (simulating the UI's gradient)
        const gradient = ctx.createLinearGradient(0, canvas.height * 0.7, 0, canvas.height);
        gradient.addColorStop(0, 'rgba(0, 0, 0, 0)');
        gradient.addColorStop(1, 'rgba(0, 0, 0, 0.6)');
        ctx.fillStyle = gradient;
        ctx.fillRect(0, canvas.height * 0.7, canvas.width, canvas.height * 0.3);

        // Reset shadow for text
        ctx.shadowColor = 'rgba(0, 0, 0, 0.8)';
        ctx.shadowBlur = 12;
        ctx.shadowOffsetX = 2;
        ctx.shadowOffsetY = 2;

        // 2. Draw Title (Bold)
        ctx.font = `bold ${titleFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
        ctx.fillStyle = 'white';
        ctx.textBaseline = 'bottom';
        const titleY = canvas.height - paddingY - (copyrightFontSize * 2.5);
        ctx.fillText(title, paddingX, titleY);

        // 3. Draw Copyright
        ctx.shadowBlur = 8;
        ctx.font = `${copyrightFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
        ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
        const copyrightY = canvas.height - paddingY - (tagFontSize * 2.0);
        ctx.fillText(copyright, paddingX, copyrightY);

        // 4. Draw Date and Variant Tags
        ctx.shadowBlur = 4;
        ctx.font = `${tagFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
        
        const tags = [date, variant || "UHD"];
        let currentTagX = paddingX;
        const tagY = canvas.height - paddingY;
        const tagPaddingH = tagFontSize * 0.8;
        const tagPaddingV = tagFontSize * 0.3;
        const tagRadius = 4;

        tags.forEach(tag => {
          const tagWidth = ctx.measureText(tag).width;
          const rectWidth = tagWidth + tagPaddingH * 2;
          const rectHeight = tagFontSize + tagPaddingV * 2;
          const rectX = currentTagX;
          const rectY = tagY - rectHeight;

          // Draw tag background
          ctx.save();
          ctx.shadowBlur = 0;
          ctx.shadowOffsetX = 0;
          ctx.shadowOffsetY = 0;
          ctx.fillStyle = 'rgba(0, 0, 0, 0.4)';
          ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
          ctx.lineWidth = 1;
          
          // Rounded rect
          ctx.beginPath();
          ctx.roundRect(rectX, rectY, rectWidth, rectHeight, tagRadius);
          ctx.fill();
          ctx.stroke();
          ctx.restore();

          // Draw tag text
          ctx.fillStyle = 'rgba(255, 255, 255, 0.6)';
          ctx.fillText(tag, rectX + tagPaddingH, tagY - tagPaddingV);
          
          currentTagX += rectWidth + 12; // Gap
        });

        const base64Data = canvas.toDataURL('image/jpeg', 0.95);
        await SubmitWatermark(base64Data);
      } catch (err) {
        console.error('Watermark render error:', err);
        await SubmitWatermark(""); // Fallback
      }
    });

    return () => {
      unregister();
      unregisterSync();
    };
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
      // Use ApplyHistory with the current key to ensure we apply the displayed image
      await ApplyHistory(currentImage.key, config?.overlay_metadata || false);
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
      const days = config?.retain_days || 0;
      if (days <= 0) {
        toast.warning('请先设置保留天数（需大于 0）');
        return;
      }
      const count = await CleanupByRetainDays(days);
      loadHistory();
      toast.info(`清理完成，删除了 ${count} 条过旧记录`);
    } catch (err) {
      toast.error('清理失败: ' + err);
    }
  };

  return (
    <TooltipProvider>
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
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="flex items-center space-x-2 bg-black/40 backdrop-blur-md p-2 px-3 rounded-full border border-white/10 pointer-events-auto">
              <Switch 
                id="auto-apply" 
                checked={config?.auto_apply || false}
                onCheckedChange={(val) => config && handleSaveConfig({ ...config, auto_apply: val })}
              />
              <Label htmlFor="auto-apply" className="text-xs font-light text-white cursor-pointer select-none">自动设置</Label>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">
            <p>获取新壁纸后自动设为桌面壁纸</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <div className="flex items-center space-x-2 bg-black/40 backdrop-blur-md p-2 px-3 rounded-full border border-white/10 pointer-events-auto">
              <Switch 
                id="overlay" 
                checked={config?.overlay_metadata || false}
                onCheckedChange={(val) => config && handleSaveConfig({ ...config, overlay_metadata: val })}
              />
              <Label htmlFor="overlay" className="text-xs font-light text-white cursor-pointer select-none">叠加水印</Label>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">
            <p>在壁纸图片上叠加标题、日期及版权信息</p>
          </TooltipContent>
        </Tooltip>
      </div>

      {/* Top Right: Actions */}
      <div className="absolute top-6 right-6 flex gap-3 z-10">
        <Drawer shouldScaleBackground={false}>
          <DrawerTrigger asChild>
            <Button variant="ghost" size="icon" className="bg-black/40 backdrop-blur-md border border-white/10 rounded-full hover:bg-black/60 text-white">
              <History className="h-5 w-5" />
            </Button>
          </DrawerTrigger>
          <DrawerContent className="h-[85vh] antialiased">
            <DrawerHeader className="border-b shrink-0">
              <div className="flex items-center justify-between">
                <DrawerTitle>壁纸历史</DrawerTitle>
                <Button variant="ghost" size="sm" onClick={handleClearHistory} className="text-destructive hover:text-destructive">
                  <Eraser className="h-4 w-4 mr-2" />
                  清空全部
                </Button>
              </div>
            </DrawerHeader>
            <ScrollArea className="flex-1 p-4">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 pb-12">
                {history.map((item) => (
                  <HistoryItemCard 
                    key={item.key} 
                    item={item} 
                    onApply={() => handleApplyHistory(item)}
                    onDelete={() => handleDeleteHistory(item.key)}
                  />
                ))}
                {history.length === 0 && (
                  <div className="col-span-full py-20 text-center text-muted-foreground font-light">
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
                 <div className={cn("flex items-center justify-between transition-opacity", config?.force_uhd && "opacity-50")}>
                    <div className="space-y-0.5">
                      <Label>优先宽高比匹配</Label>
                      <p className="text-[0.7rem] text-muted-foreground">优先选择比例接近屏幕的分辨率</p>
                    </div>
                    <Switch 
                      disabled={config?.force_uhd}
                      checked={config?.force_uhd ? false : (config?.prefer_aspect_match || false)}
                      onCheckedChange={(val) => config && setConfig({ ...config, prefer_aspect_match: val })}
                    />
                 </div>
                 <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>强制 4K (UHD)</Label>
                      <p className="text-[0.7rem] text-muted-foreground">忽略比例，存在 UHD 则必选</p>
                    </div>
                    <Switch 
                      checked={config?.force_uhd || false}
                      onCheckedChange={(val) => config && setConfig({ ...config, force_uhd: val })}
                    />
                 </div>
              </div>

              <div className="flex gap-2 pt-2">
                <Button variant="secondary" className="flex-1" onClick={() => OpenDataDir()}>数据目录</Button>
                <Button variant="secondary" className="flex-1" onClick={() => OpenLogsDir()}>日志目录</Button>
              </div>
            </div>
            <DialogFooter className="flex flex-row justify-between items-center">
              <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive hover:bg-destructive/10" onClick={() => Quit()}>
                彻底退出应用
              </Button>
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
    </TooltipProvider>
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
    <div className="group relative overflow-hidden rounded-xl border bg-card text-card-foreground shadow-sm hover:shadow-xl transition-all duration-300 ease-in-out">
      <div className="aspect-video w-full overflow-hidden bg-muted">
        {thumb ? (
          <img 
            src={thumb} 
            alt={item.title} 
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-110" 
          />
        ) : (
          <div className="h-full w-full flex items-center justify-center">
            <ImageIcon className="h-8 w-8 text-muted-foreground/50" />
          </div>
        )}
      </div>
      
      {/* Information Overlay on Hover */}
      <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex flex-col justify-end p-4">
        <div className="translate-y-4 group-hover:translate-y-0 transition-transform duration-300">
          <h4 className="text-sm font-bold text-white mb-1 line-clamp-1">{item.title}</h4>
          <p className="text-[10px] text-white/70 line-clamp-2 mb-2 leading-tight h-8">
            {item.copyright}
          </p>
          <div className="flex items-center gap-2 mb-3">
             <span className="text-[9px] bg-white/20 px-1.5 py-0.5 rounded text-white/90 backdrop-blur-sm border border-white/10">
               {item.date}
             </span>
             <span className="text-[9px] bg-white/20 px-1.5 py-0.5 rounded text-white/90 backdrop-blur-sm border border-white/10">
               {item.chosen_variant}
             </span>
          </div>
          <div className="flex gap-2">
            <Button 
              size="sm" 
              variant="secondary" 
              className="h-8 flex-1 bg-white text-black hover:bg-white/90 text-[11px]" 
              onClick={onApply}
            >
              <CheckCircle2 className="h-3.5 w-3.5 mr-1" />
              设为壁纸
            </Button>
            <Button 
              size="sm" 
              variant="destructive" 
              className="h-8 px-2" 
              onClick={onDelete}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
