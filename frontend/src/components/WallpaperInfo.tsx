import { RefreshCw, Image as ImageIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { store } from '../../wailsjs/go/models';

interface WallpaperInfoProps {
  currentImage: store.HistoryItem | null;
  loading: boolean;
  onRefresh: () => void;
  onApply: () => void;
}

export function WallpaperInfo({ currentImage, loading, onRefresh, onApply }: WallpaperInfoProps) {
  return (
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
          onClick={onRefresh}
          disabled={loading}
        >
          <RefreshCw className={cn("h-5 w-5 mr-2", loading && "animate-spin")} />
          刷新
        </Button>
        <Button 
          size="lg" 
          className="rounded-full px-8 shadow-xl bg-white text-black hover:bg-white/90 hover:scale-105 transition-transform"
          onClick={onApply}
          disabled={loading || !currentImage}
        >
          <ImageIcon className="h-5 w-5 mr-2" />
          设为壁纸
        </Button>
      </div>
    </div>
  );
}
