import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';

interface ResetDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export function ResetDialog({ open, onOpenChange, onConfirm }: ResetDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle className="text-destructive">确认重置应用？</DialogTitle>
        </DialogHeader>
        <div className="py-4 text-sm text-muted-foreground">
          此操作将物理删除所有已下载的历史壁纸、数据记录并将应用配置恢复为初始默认状态。该过程不可逆，请确认是否继续。
        </div>
        <DialogFooter className="gap-2 sm:gap-0">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>取消操作</Button>
          <Button variant="destructive" onClick={onConfirm}>确认重置</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
