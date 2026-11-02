#import <Cocoa/Cocoa.h>

extern void goSelectAll(void);
extern void goCopyAll(void);

// Called from Go: see tabbing_darwin.go.
void disableWindowTabbing(void) {
    [NSWindow setAllowsAutomaticWindowTabbing:NO];
}

@interface SelectAllTarget : NSObject
- (void)selectAllClicked:(id)sender;
- (void)copyAllClicked:(id)sender;
@end

@implementation SelectAllTarget
- (void)selectAllClicked:(id)sender {
    goSelectAll();
}
- (void)copyAllClicked:(id)sender {
    goCopyAll();
}
@end

// Edit > Select All selects the whole web view by default. Send it to the page instead, which
// knows whether the bookmarks, the page or the chat is meant.
void routeSelectAll(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        static SelectAllTarget *target;
        if (!target) target = [SelectAllTarget new];
        NSMenu *edit = [[[NSApp mainMenu] itemWithTitle:@"Edit"] submenu];
        NSInteger copyAt = -1;
        BOOL haveCopyAll = NO;
        for (NSMenuItem *item in edit.itemArray) {
            if (item.action == @selector(selectAll:)) {
                item.target = target;
                item.action = @selector(selectAllClicked:);
            }
            if (item.action == @selector(copy:)) copyAt = [edit indexOfItem:item];
            if (item.action == @selector(copyAllClicked:)) haveCopyAll = YES;
        }
        // Copy All (the page's markdown) goes right below Copy.
        if (copyAt >= 0 && !haveCopyAll) {
            NSMenuItem *all = [[NSMenuItem alloc] initWithTitle:@"Copy All" action:@selector(copyAllClicked:) keyEquivalent:@"c"];
            all.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;
            all.target = target;
            [edit insertItem:all atIndex:copyAt + 1];
        }
    });
}
